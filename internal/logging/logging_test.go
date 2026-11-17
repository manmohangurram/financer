package logging

import (
	"fmt"
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

func TestRotatesAtSizeAndKeepsThreeFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "financer.log")
	rf, err := newRotatingFile(path, 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()

	// Each line is ~20 bytes; 60 writes force several rotations.
	for i := 0; i < 60; i++ {
		if _, err := rf.Write([]byte(fmt.Sprintf("line-%03d-abcdefghij\n", i))); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
		if info, _ := e.Info(); info.Size() > 100 {
			t.Errorf("%s exceeds the 100-byte limit: %d", e.Name(), info.Size())
		}
	}
	if len(names) > 3 {
		t.Fatalf("kept %d files, want at most 3: %v", len(names), names)
	}
	// The newest content must survive, and the oldest lines must be gone.
	newest, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newest), "line-059") {
		t.Fatalf("active file missing newest line: %q", newest)
	}
	oldest := filepath.Join(dir, "financer.log.2")
	if body, err := os.ReadFile(oldest); err == nil && strings.Contains(string(body), "line-000") {
		t.Fatal("the oldest rotated file should have been deleted")
	}
}

func TestOversizedFileRotatesOnOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "financer.log")
	if err := os.WriteFile(path, make([]byte, 200), 0o644); err != nil {
		t.Fatal(err)
	}
	rf, err := newRotatingFile(path, 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("oversized file was not rotated: %v", err)
	}
}
