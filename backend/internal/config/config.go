// Package config reads settings from the environment, with defaults
// suitable for local development via docker-compose.yml.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"

	"bigroup-backend/internal/auth"
)

type Config struct {
	HTTPAddr      string
	PgDSN         string
	RedisAddr     string
	RedisPassword string
	CacheTTL      time.Duration
	MigrateOnRun  bool
	FrontOrigin   string
	CookieSecure  bool
	AdminLogin    string
	AdminPassword string
	// AppEnv marks the environment ("dev", "prod", ...). Only "dev" allows MigrateReset
	// and relaxes the production checks in Validate.
	AppEnv string
	// MigrateReset drops and recreates the public schema before migrating (dev only).
	MigrateReset bool
	// TrustedProxies are the CIDRs whose X-Forwarded-For is honoured (TRUSTED_PROXIES).
	TrustedProxies []netip.Prefix
}

// IsDev reports APP_ENV=dev.
func (c Config) IsDev() bool { return c.AppEnv == "dev" }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads the environment. COOKIE_SECURE defaults to 1 unless APP_ENV=dev.
func Load() (Config, error) {
	appEnv := os.Getenv("APP_ENV")
	secureDef := "1"
	if appEnv == "dev" {
		secureDef = "0"
	}
	trusted, err := auth.ParseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return Config{}, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	return Config{
		HTTPAddr:       getenv("HTTP_ADDR", ":8080"),
		PgDSN:          getenv("PG_DSN", "postgres://postgres:postgres@localhost:5432/bigroup?sslmode=disable"),
		RedisAddr:      getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		CacheTTL:       10 * time.Minute,
		MigrateOnRun:   getenv("MIGRATE", "1") == "1",
		FrontOrigin:    getenv("FRONT_ORIGIN", "http://localhost:5173"),
		CookieSecure:   getenv("COOKIE_SECURE", secureDef) == "1",
		AdminLogin:     getenv("ADMIN_LOGIN", "admin"),
		AdminPassword:  getenv("ADMIN_PASSWORD", ""),
		AppEnv:         appEnv,
		MigrateReset:   os.Getenv("MIGRATE_RESET") == "1",
		TrustedProxies: trusted,
	}, nil
}

// minAdminPassword is the shortest bootstrap admin password accepted outside dev.
const minAdminPassword = 12

// weakAdminPasswords are refused outside dev regardless of length.
var weakAdminPasswords = map[string]bool{"admin": true, "admin123": true}

// Validate enforces the production checks; APP_ENV=dev skips them. It runs before
// any connection, so a misconfigured server never starts.
func (c Config) Validate() error {
	if c.IsDev() {
		return nil
	}
	var errs []error
	if !strings.HasPrefix(strings.ToLower(c.FrontOrigin), "https://") {
		errs = append(errs, fmt.Errorf("FRONT_ORIGIN must be https:// when APP_ENV=%q (got %q)", c.AppEnv, c.FrontOrigin))
	}
	// An empty ADMIN_PASSWORD only skips the bootstrap; a set one must be strong.
	if c.AdminPassword != "" && (len(c.AdminPassword) < minAdminPassword || weakAdminPasswords[strings.ToLower(c.AdminPassword)]) {
		errs = append(errs, fmt.Errorf("ADMIN_PASSWORD is too weak for APP_ENV=%q: use %d+ characters, not admin/admin123", c.AppEnv, minAdminPassword))
	}
	return errors.Join(errs...)
}
