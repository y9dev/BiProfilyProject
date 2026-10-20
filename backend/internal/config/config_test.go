package config

import (
	"strings"
	"testing"
)

func TestCookieSecureDefault(t *testing.T) {
	for _, tc := range []struct {
		env, secure string
		want        bool
	}{
		{"dev", "", false},
		{"prod", "", true},
		{"", "", true},
		{"dev", "1", true},
		{"prod", "0", false},
	} {
		t.Setenv("APP_ENV", tc.env)
		t.Setenv("COOKIE_SECURE", tc.secure)
		c, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if c.CookieSecure != tc.want {
			t.Errorf("APP_ENV=%q COOKIE_SECURE=%q: secure=%v, want %v", tc.env, tc.secure, c.CookieSecure, tc.want)
		}
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8, 172.18.0.1")
	c, err := Load()
	if err != nil || len(c.TrustedProxies) != 2 {
		t.Fatalf("got %v %v", c.TrustedProxies, err)
	}
	t.Setenv("TRUSTED_PROXIES", "not-a-cidr")
	if _, err := Load(); err == nil {
		t.Fatal("bad TRUSTED_PROXIES accepted")
	}
}

func TestLoadRedisPassword(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "s3cret")
	c, _ := Load()
	if c.RedisPassword != "s3cret" {
		t.Fatalf("redis password %q", c.RedisPassword)
	}
}

func TestValidate(t *testing.T) {
	strong := "correct-horse-battery"
	for _, tc := range []struct {
		name    string
		c       Config
		wantErr string
	}{
		{"dev allows anything", Config{AppEnv: "dev", FrontOrigin: "http://localhost:5173", AdminPassword: "admin123"}, ""},
		{"prod ok", Config{AppEnv: "prod", FrontOrigin: "https://bi.example.com", AdminPassword: strong}, ""},
		{"prod no admin password skips bootstrap", Config{AppEnv: "prod", FrontOrigin: "https://bi.example.com"}, ""},
		{"prod http origin", Config{AppEnv: "prod", FrontOrigin: "http://bi.example.com", AdminPassword: strong}, "FRONT_ORIGIN"},
		{"empty env is strict", Config{FrontOrigin: "http://localhost:5173"}, "FRONT_ORIGIN"},
		{"prod short password", Config{AppEnv: "prod", FrontOrigin: "https://x", AdminPassword: "short-pw"}, "ADMIN_PASSWORD"},
		{"prod admin123", Config{AppEnv: "prod", FrontOrigin: "https://x", AdminPassword: "admin123"}, "ADMIN_PASSWORD"},
		{"prod admin", Config{AppEnv: "staging", FrontOrigin: "https://x", AdminPassword: "admin"}, "ADMIN_PASSWORD"},
	} {
		err := tc.c.Validate()
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: unexpected %v", tc.name, err)
		case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
			t.Errorf("%s: err %v, want %q", tc.name, err, tc.wantErr)
		}
	}
}
