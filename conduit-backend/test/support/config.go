package support

import (
	"path/filepath"
	"runtime"
	"testing"

	"conduit/backend/internal/config"
)

const TestAuthSecret = "test-secret-value-with-more-than-32-chars"

func ProjectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		"AUTH_SECRET":   TestAuthSecret,
		"DATABASE_URL":  "postgres://127.0.0.1:5432/conduit",
		"APP_HOST":      "127.0.0.1",
		"APP_PORT":      "0",
		"BCRYPT_ROUNDS": "4",
		"LOG_SILENT":    "true",
		"LOG_TO_FILE":   "false",
	}
	for key, value := range overrides {
		env[key] = value
	}
	return env
}

func TestConfig(tb testing.TB, overrides map[string]string) config.Config {
	tb.Helper()
	env := TestEnv(overrides)
	cfg, err := config.Load(func(key string) string { return env[key] })
	if err != nil {
		tb.Fatalf("invalid test config: %v", err)
	}
	return cfg
}
