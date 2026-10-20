package invitations

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"bigroup-backend/internal/migrate"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/testutil"
)

// Needs Postgres (TEST_PG_DSN); skips otherwise. Run with -p 1 (shared database).
func redeemPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/bigroup?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil || pool.Ping(ctx) != nil {
		t.Skipf("no postgres (%v)", err)
	}
	if _, err := migrate.Up(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`TRUNCATE audit_log, attempt_scores, attempt_answers, attempts, option_effects, options, questions,
		 assessments, users, invitations, departments RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func redeemOnce(pool *pgxpool.Pool, code string) error {
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := New(pool).Redeem(ctx, tx, code); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func TestRedeem_Basics(t *testing.T) {
	pool := redeemPool(t)
	adm := testutil.User(t, pool, "boss", "secret-123", model.RoleAdmin, true)
	past := time.Now().Add(-time.Minute)
	testutil.Invitation(t, pool, adm.ID, "GOODGOODGOOD", testutil.InviteOpts{Max: 1})
	testutil.Invitation(t, pool, adm.ID, "OLDOLDOLDOLD", testutil.InviteOpts{ExpiresAt: &past})
	testutil.Invitation(t, pool, adm.ID, "REVREVREVREV", testutil.InviteOpts{Revoked: true})

	repo := New(pool)
	for code, want := range map[string]bool{"GOODGOODGOOD": true, "OLDOLDOLDOLD": false, "REVREVREVREV": false, "UNKNOWNUNKNO": false} {
		if got, err := repo.IsRedeemable(context.Background(), code); err != nil || got != want {
			t.Errorf("IsRedeemable(%s) = %v, %v; want %v", code, got, err, want)
		}
	}
	if err := redeemOnce(pool, "GOODGOODGOOD"); err != nil {
		t.Fatalf("first redeem: %v", err)
	}
	for _, code := range []string{"GOODGOODGOOD", "OLDOLDOLDOLD", "REVREVREVREV", "UNKNOWNUNKNO"} {
		if err := redeemOnce(pool, code); err != ErrInviteInvalid {
			t.Errorf("redeem %s: %v, want ErrInviteInvalid", code, err)
		}
	}
}

// TestRedeem_ConcurrentAtLimit: 20 goroutines against max=5 admit exactly 5 (AC-1).
func TestRedeem_ConcurrentAtLimit(t *testing.T) {
	pool := redeemPool(t)
	adm := testutil.User(t, pool, "boss", "secret-123", model.RoleAdmin, true)
	id := testutil.Invitation(t, pool, adm.ID, "RACERACERACE", testutil.InviteOpts{Max: 5})

	var ok, invalid int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			switch err := redeemOnce(pool, "RACERACERACE"); err {
			case nil:
				atomic.AddInt32(&ok, 1)
			case ErrInviteInvalid:
				atomic.AddInt32(&invalid, 1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	var count int
	_ = pool.QueryRow(context.Background(), `SELECT registrations_count FROM invitations WHERE id=$1`, id).Scan(&count)
	if ok != 5 || invalid != 15 || count != 5 {
		t.Fatalf("ok=%d invalid=%d count=%d, want 5/15/5", ok, invalid, count)
	}
}
