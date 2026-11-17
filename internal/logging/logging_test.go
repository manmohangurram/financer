package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupWritesToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "financer.log")
	closeLogs, err := Setup("debug", path)
	if err != nil {
		t.Fatal(err)
	}
	slog.Info("hello", "kind", "test")
	closeLogs()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "hello") || !strings.Contains(string(body), "kind=test") {
		t.Fatalf("log file = %q", body)
	}
}

func TestSetupRejectsUnwritablePath(t *testing.T) {
	// A directory cannot be opened as an append-only file.
	dir := t.TempDir()
	if _, err := Setup("info", dir); err == nil {
		t.Fatal("expected an error opening a directory as the log file")
	}
}

func TestSetupAcceptsUnknownLevel(t *testing.T) {
	closeLogs, err := Setup("verbose-nonsense", "")
	if err != nil {
		t.Fatal(err)
	}
	closeLogs()
}
