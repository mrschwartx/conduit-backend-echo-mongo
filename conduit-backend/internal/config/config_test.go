package config

import (
	"strings"
	"testing"
	"time"
)

const validSecret = "0123456789abcdef0123456789abcdef"

func lookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func baseEnv() map[string]string {
	return map[string]string{"AUTH_SECRET": validSecret, "DATABASE_URL": "postgres://localhost/conduit"}
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(lookup(baseEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Port != 8080 || cfg.App.Host != "0.0.0.0" || cfg.App.Name != "conduit-api" {
		t.Fatalf("unexpected app defaults: %+v", cfg.App)
	}
	if cfg.App.IsProduction || cfg.Logging.Level != "debug" || !cfg.Logging.ToFile || cfg.Logging.Silent {
		t.Fatalf("unexpected logging defaults: %+v", cfg.Logging)
	}
	if cfg.Auth.BcryptRounds != 10 || cfg.Auth.TokenTTL != 7*24*time.Hour {
		t.Fatalf("unexpected auth defaults: %+v", cfg.Auth)
	}
	if cfg.Database.PoolMax != 10 || cfg.Database.Timeout != 30*time.Second {
		t.Fatalf("unexpected database defaults: %+v", cfg.Database)
	}
	if cfg.ClientURL != "http://localhost:3000" {
		t.Fatalf("unexpected client url %q", cfg.ClientURL)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	env := baseEnv()
	env["APP_LEVEL"] = "production"
	env["APP_PORT"] = "9001"
	env["DATABASE_POOL_MAX"] = "3"
	env["DATABASE_TIMEOUT"] = "500"
	env["BCRYPT_ROUNDS"] = "12"
	env["LOG_TO_FILE"] = "false"
	env["LOG_SILENT"] = "YES"
	env["AUTH_TOKEN_TTL"] = "2h"
	cfg, err := Load(lookup(env))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.App.IsProduction || cfg.Logging.Level != "info" || cfg.App.Port != 9001 {
		t.Fatalf("unexpected app config: %+v %+v", cfg.App, cfg.Logging)
	}
	if cfg.Database.PoolMax != 3 || cfg.Database.Timeout != 500*time.Millisecond || cfg.Auth.BcryptRounds != 12 {
		t.Fatalf("unexpected numeric config: %+v %+v", cfg.Database, cfg.Auth)
	}
	if cfg.Logging.ToFile || !cfg.Logging.Silent || cfg.Auth.TokenTTL != 2*time.Hour {
		t.Fatalf("unexpected flags: %+v %+v", cfg.Logging, cfg.Auth)
	}
}

func TestLoadFallsBackOnInvalidNumbers(t *testing.T) {
	env := baseEnv()
	env["APP_PORT"] = "not-a-number"
	env["BCRYPT_ROUNDS"] = ""
	cfg, err := Load(lookup(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Port != 8080 || cfg.Auth.BcryptRounds != 10 {
		t.Fatalf("expected fallbacks, got %+v %+v", cfg.App, cfg.Auth)
	}
}

func TestLoadRejectsInvalidSecurityConfig(t *testing.T) {
	cases := map[string]struct {
		mutate func(map[string]string)
		want   string
	}{
		"missing secret":   {func(e map[string]string) { delete(e, "AUTH_SECRET") }, "AUTH_SECRET"},
		"short secret":     {func(e map[string]string) { e["AUTH_SECRET"] = "short" }, "at least 32"},
		"missing database": {func(e map[string]string) { delete(e, "DATABASE_URL") }, "DATABASE_URL"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			env := baseEnv()
			tc.mutate(env)
			_, err := Load(lookup(env))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestToInteger(t *testing.T) {
	cases := map[string]int{"42": 42, " 7 ": 7, "12abc": 12, "-3": -3, "abc": 99, "": 99}
	for input, want := range cases {
		if got := toInteger(input, 99); got != want {
			t.Errorf("toInteger(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestToTokenTTL(t *testing.T) {
	cases := map[string]time.Duration{
		"":                7 * 24 * time.Hour,
		"garbage":         7 * 24 * time.Hour,
		"0":               7 * 24 * time.Hour,
		"-5m":             7 * 24 * time.Hour,
		"3600":            time.Hour,
		"45s":             45 * time.Second,
		"30m":             30 * time.Minute,
		"2h":              2 * time.Hour,
		" 7D ":            7 * 24 * time.Hour,
		"2w":              14 * 24 * time.Hour,
		"99999999999999d": 7 * 24 * time.Hour,
	}
	for input, want := range cases {
		if got := toTokenTTL(input); got != want {
			t.Errorf("toTokenTTL(%q) = %v, want %v", input, got, want)
		}
	}
}
