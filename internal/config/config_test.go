package config

import (
	"os"
	"testing"
)

func TestFromEnvRequiresJWTSecret(t *testing.T) {
	os.Unsetenv("FINANCER_JWT_SECRET")
	_, err := FromEnv()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET missing")
	}
}

func TestFromEnvDefaults(t *testing.T) {
	os.Setenv("FINANCER_JWT_SECRET", "secret")
	defer os.Unsetenv("FINANCER_JWT_SECRET")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Addr != "0.0.0.0:8080" {
		t.Errorf("addr = %q", cfg.Server.Addr)
	}
	if cfg.Server.Timezone != "Asia/Kolkata" {
		t.Errorf("timezone = %q", cfg.Server.Timezone)
	}
	if cfg.Storage.SQLite.WritePoolSize != 1 || cfg.Storage.SQLite.ReadPoolSize != 5 {
		t.Errorf("pools = %d/%d", cfg.Storage.SQLite.WritePoolSize, cfg.Storage.SQLite.ReadPoolSize)
	}
	if cfg.Server.LogLevel != "info" || cfg.Server.LogFile != "" {
		t.Errorf("logging defaults = %q/%q", cfg.Server.LogLevel, cfg.Server.LogFile)
	}
}

func TestSecretKeyValidation(t *testing.T) {
	if _, err := validateSecretKey(""); err == nil {
		t.Fatal("expected error for empty key")
	}
	if _, err := validateSecretKey("z"); err == nil {
		t.Fatal("expected error for non-hex")
	}
	if _, err := validateSecretKey(repeat("a", 63)); err == nil {
		t.Fatal("expected error for 63 chars")
	}
	got, err := validateSecretKey(repeat("A", 64))
	if err != nil {
		t.Fatal(err)
	}
	if got != repeat("a", 64) {
		t.Errorf("key not lowercased: %q", got)
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
