// Точка входа: конфиг → Postgres → Redis → миграции → HTTP.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"bigroup-backend/internal/admin"
	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/config"
	"bigroup-backend/internal/httpapi"
	"bigroup-backend/internal/migrate"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/store"
	"bigroup-backend/internal/users"
)

func mustPostgres(ctx context.Context, dsn string) *pgxpool.Pool {
	var pool *pgxpool.Pool
	var err error
	for i := 0; i < 30; i++ {
		pool, err = store.NewPostgres(ctx, dsn)
		if err == nil {
			return pool
		}
		log.Printf("postgres not ready (%v), retry %d/30", err, i+1)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("postgres: %v", err)
	return nil
}

func mustRedis(ctx context.Context, addr, password string) *redis.Client {
	var rdb *redis.Client
	var err error
	for i := 0; i < 30; i++ {
		rdb, err = store.NewRedis(ctx, addr, password)
		if err == nil {
			return rdb
		}
		log.Printf("redis not ready (%v), retry %d/30", err, i+1)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("redis: %v", err)
	return nil
}

// bootstrapAdmin создаёт первого администратора из env, если users пуст.
// Без ADMIN_PASSWORD ничего не создаётся (безопасный дефолт). Outside APP_ENV=dev a
// weak ADMIN_PASSWORD (< 12 chars, admin, admin123) is refused earlier by cfg.Validate.
func bootstrapAdmin(ctx context.Context, repo users.Repository, cfg config.Config) {
	n, err := repo.Count(ctx)
	if err != nil {
		log.Fatalf("users count: %v", err)
	}
	if n > 0 || cfg.AdminPassword == "" {
		return
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		log.Fatalf("hash admin password: %v", err)
	}
	u, err := repo.Create(ctx, model.User{
		Login: cfg.AdminLogin, FullName: "Администратор", Role: model.RoleAdmin,
	}, hash)
	if err != nil {
		log.Fatalf("create admin: %v", err)
	}
	log.Printf("bootstrap admin %q created (id=%d)", u.Login, u.ID)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	// Production checks (https origin, strong admin password) run before any connection.
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()

	// Checked before any DB connection: a refused reset never touches data.
	if err := migrate.CheckReset(cfg.MigrateReset, cfg.AppEnv); err != nil {
		log.Fatalf("migrate: %v (APP_ENV=%q)", err, cfg.AppEnv)
	}

	pool := mustPostgres(ctx, cfg.PgDSN)
	defer pool.Close()
	rdb := mustRedis(ctx, cfg.RedisAddr, cfg.RedisPassword)
	defer func() { _ = rdb.Close() }()

	if cfg.MigrateReset {
		if err := migrate.Reset(ctx, pool); err != nil {
			log.Fatalf("migrate reset: %v", err)
		}
		log.Println("migrate: MIGRATE_RESET=1 (APP_ENV=dev): public schema dropped and recreated")
	}
	if cfg.MigrateOnRun || cfg.MigrateReset {
		if _, err := migrate.Up(ctx, pool); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	}

	userRepo := users.New(pool)
	bootstrapAdmin(ctx, userRepo, cfg)

	srv := httpapi.New(httpapi.Deps{
		Pool: pool, Redis: rdb, Users: userRepo, Admin: admin.New(pool),
		Sessions: auth.NewManager(rdb, cfg.CookieSecure), FrontOrigin: cfg.FrontOrigin,
		TrustedProxies: cfg.TrustedProxies,
	})

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("backend on %s (APP_ENV=%q, secure cookies=%v, trusted proxies=%d)", cfg.HTTPAddr, cfg.AppEnv, cfg.CookieSecure, len(cfg.TrustedProxies))
	log.Fatal(httpSrv.ListenAndServe())
}
