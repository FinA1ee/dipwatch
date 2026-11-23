// Package config reads service configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
)

// minTokenLen keeps the gateway-to-core token from being trivially guessable.
const minTokenLen = 32

type Config struct {
	Addr           string
	DatabaseURL    string
	InternalToken  string
	MigrateOnStart bool
}

// Load reads the config for core-api. The worker uses LoadDatabaseURL.
func Load() (Config, error) {
	dbURL, err := LoadDatabaseURL()
	if err != nil {
		return Config{}, err
	}
	token := os.Getenv("CORE_INTERNAL_TOKEN")
	if len(token) < minTokenLen {
		return Config{}, fmt.Errorf("CORE_INTERNAL_TOKEN must be at least %d characters", minTokenLen)
	}
	return Config{
		Addr:           getenv("CORE_ADDR", ":8000"),
		DatabaseURL:    dbURL,
		InternalToken:  token,
		MigrateOnStart: getenv("CORE_MIGRATE_ON_START", "true") == "true",
	}, nil
}

func LoadDatabaseURL() (string, error) {
	u := os.Getenv("DATABASE_URL")
	if u == "" {
		return "", errors.New("DATABASE_URL is required")
	}
	return u, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
