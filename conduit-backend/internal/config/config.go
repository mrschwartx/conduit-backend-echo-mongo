package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	minSecretLength = 32
	defaultTokenTTL = 7 * 24 * time.Hour
)

var (
	leadingInteger = regexp.MustCompile(`^[+-]?\d+`)
	ttlPattern     = regexp.MustCompile(`^(\d+)\s*(s|m|h|d|w)?$`)
)

type Config struct {
	App       App
	ClientURL string
	Database  Database
	Auth      Auth
	Logging   Logging
}

type App struct {
	Name         string
	Host         string
	Port         int
	Level        string
	IsProduction bool
}

type Database struct {
	URL     string
	PoolMax int
	Timeout time.Duration
}

type Auth struct {
	Secret       string
	TokenTTL     time.Duration
	BcryptRounds int
}

type Logging struct {
	Level  string
	ToFile bool
	Dir    string
	Silent bool
}

func Load(getenv func(string) string) (Config, error) {
	level := valueOr(getenv("APP_LEVEL"), "development")
	isProduction := level == "production"

	secret, err := requireValue(getenv, "AUTH_SECRET")
	if err != nil {
		return Config{}, err
	}
	if utf8.RuneCountInString(secret) < minSecretLength {
		return Config{}, fmt.Errorf("AUTH_SECRET must be at least %d characters long", minSecretLength)
	}

	databaseURL, err := requireValue(getenv, "DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	logDir, err := filepath.Abs(valueOr(getenv("LOG_DIR"), "logs"))
	if err != nil {
		return Config{}, err
	}

	defaultLogLevel := "debug"
	if isProduction {
		defaultLogLevel = "info"
	}

	return Config{
		App: App{
			Name:         valueOr(getenv("APP_NAME"), "conduit-api"),
			Host:         valueOr(getenv("APP_HOST"), "0.0.0.0"),
			Port:         toInteger(getenv("APP_PORT"), 8080),
			Level:        level,
			IsProduction: isProduction,
		},
		ClientURL: valueOr(getenv("CLIENT_URL"), "http://localhost:3000"),
		Database: Database{
			URL:     databaseURL,
			PoolMax: toInteger(getenv("DATABASE_POOL_MAX"), 10),
			Timeout: time.Duration(toInteger(getenv("DATABASE_TIMEOUT"), 30000)) * time.Millisecond,
		},
		Auth: Auth{
			Secret:       secret,
			TokenTTL:     toTokenTTL(getenv("AUTH_TOKEN_TTL")),
			BcryptRounds: toInteger(getenv("BCRYPT_ROUNDS"), 10),
		},
		Logging: Logging{
			Level:  valueOr(getenv("LOG_LEVEL"), defaultLogLevel),
			ToFile: toBoolean(getenv("LOG_TO_FILE"), true),
			Dir:    logDir,
			Silent: toBoolean(getenv("LOG_SILENT"), false),
		},
	}, nil
}

func requireValue(getenv func(string) string, key string) (string, error) {
	value := getenv(key)
	if value == "" {
		return "", errors.New("Missing required environment variable: " + key)
	}
	return value, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func toInteger(value string, fallback int) int {
	match := leadingInteger.FindString(strings.TrimSpace(value))
	if match == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(match)
	if err != nil {
		return fallback
	}
	return parsed
}

func toBoolean(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func toTokenTTL(value string) time.Duration {
	match := ttlPattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(value)))
	if match == nil {
		return defaultTokenTTL
	}
	amount, err := strconv.Atoi(match[1])
	if err != nil || amount <= 0 {
		return defaultTokenTTL
	}
	unit := map[string]time.Duration{
		"":  time.Second,
		"s": time.Second,
		"m": time.Minute,
		"h": time.Hour,
		"d": 24 * time.Hour,
		"w": 7 * 24 * time.Hour,
	}[match[2]]
	ttl := time.Duration(amount) * unit
	if ttl <= 0 {
		return defaultTokenTTL
	}
	return ttl
}
