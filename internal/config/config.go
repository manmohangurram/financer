// Package config reads FINANCER_* environment variables.
// Every setting has a built-in default; blank/unset vars keep the default.
// FINANCER_JWT_SECRET is required.
package config

import (
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server  Server
	Storage Storage
}

type Server struct {
	Addr              string
	DataDir           string
	StaticDir         string
	DomainURL         string
	JWTSecret         string
	Timezone          string
	SecretKey         *string
	MailPollSecs      int
	AllowInsecureHTTP bool
}

type Storage struct {
	SQLite Sqlite
}

type Sqlite struct {
	Path          string
	JournalMode   string
	Synchronous   string
	BusyTimeoutMs int
	ForeignKeys   bool
	PageSize      int
	WritePoolSize int
	ReadPoolSize  int
}

func Default() Config {
	return Config{
		Server: Server{
			Addr:              "0.0.0.0:8080",
			DataDir:           "/data",
			StaticDir:         "frontend/dist",
			DomainURL:         "",
			JWTSecret:         "",
			Timezone:          "Asia/Kolkata",
			MailPollSecs:      300,
			AllowInsecureHTTP: false,
		},
		Storage: Storage{
			SQLite: Sqlite{
				Path:          "/data/financer.db",
				JournalMode:   "wal",
				Synchronous:   "normal",
				BusyTimeoutMs: 5000,
				ForeignKeys:   true,
				PageSize:      0,
				WritePoolSize: 1,
				ReadPoolSize:  5,
			},
		},
	}
}

func FromEnv() (Config, error) {
	cfg := Default()
	if v := os.Getenv("FINANCER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("FINANCER_DATA_DIR"); v != "" {
		cfg.Server.DataDir = v
	}
	if v := os.Getenv("FINANCER_STATIC_DIR"); v != "" {
		cfg.Server.StaticDir = v
	}
	if v := os.Getenv("FINANCER_DOMAIN_URL"); v != "" {
		cfg.Server.DomainURL = v
	}
	if v := os.Getenv("FINANCER_JWT_SECRET"); v != "" {
		cfg.Server.JWTSecret = v
	}
	if v := os.Getenv("FINANCER_TIMEZONE"); v != "" {
		cfg.Server.Timezone = v
	}
	if v := os.Getenv("FINANCER_SECRET_KEY"); v != "" {
		k, err := validateSecretKey(v)
		if err != nil {
			return Config{}, err
		}
		cfg.Server.SecretKey = &k
	}
	if v := os.Getenv("FINANCER_ALLOW_INSECURE_HTTP"); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes":
			cfg.Server.AllowInsecureHTTP = true
		}
	}
	if v := os.Getenv("FINANCER_MAIL_POLL_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 30 {
			cfg.Server.MailPollSecs = n
		}
	}
	if v := os.Getenv("FINANCER_SQLITE_PATH"); v != "" {
		cfg.Storage.SQLite.Path = v
	}
	if cfg.Server.JWTSecret == "" {
		return Config{}, errors.New("FINANCER_JWT_SECRET is required — set it to a stable secret (e.g. `openssl rand -hex 32`)")
	}
	if cfg.Server.SecretKey != nil {
		lower := strings.ToLower(*cfg.Server.SecretKey)
		cfg.Server.SecretKey = &lower
	}
	return cfg, nil
}

func validateSecretKey(v string) (string, error) {
	b, err := hex.DecodeString(v)
	if err != nil || len(b) != 32 {
		return "", errors.New("FINANCER_SECRET_KEY must be 32 bytes of hex (64 characters)")
	}
	return strings.ToLower(v), nil
}
