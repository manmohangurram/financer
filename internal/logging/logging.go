// Package logging configures the process-wide slog logger from FINANCER_LOG_*
// settings. The default is human-readable text on stderr; setting a file path
// appends there instead, rotating at 10 MB and keeping at most three files
// (the active file plus two older ones).
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Rotation limits for a file-backed log. The active file plus the two rotated
// files are the only ones kept: older files are deleted automatically.
const (
	MaxLogBytes int64 = 10 << 20 // 10 MB
	MaxLogFiles       = 3        // active + 2 rotated
)

// Setup installs the default logger and returns a close function. An unknown
// level falls back to info; an unwritable log file returns an error.
func Setup(level, file string) (func(), error) {
	var (
		w       io.Writer = os.Stderr
		closeFn           = func() {}
	)
	if strings.TrimSpace(file) != "" {
		if dir := filepath.Dir(file); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("creating log directory: %w", err)
			}
		}
		rf, err := newRotatingFile(file, MaxLogBytes, MaxLogFiles)
		if err != nil {
			return nil, fmt.Errorf("opening log file: %w", err)
		}
		w = rf
		closeFn = func() { _ = rf.Close() }
	}

	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl})))
	return closeFn, nil
}

// rotatingFile is an append-only file that rotates itself once a write would
// exceed maxBytes. Files are named path, path.1, path.2, ...; only maxFiles
// exist at once, and the oldest is deleted on rotation.
type rotatingFile struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	maxFiles int
	file     *os.File
	size     int64
}

func newRotatingFile(path string, maxBytes int64, maxFiles int) (*rotatingFile, error) {
	r := &rotatingFile{path: path, maxBytes: maxBytes, maxFiles: maxFiles}
	if err := r.open(); err != nil {
		return nil, err
	}
	// A previous run may have left the active file at or above the limit.
	if r.size >= r.maxBytes {
		if err := r.rotate(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *rotatingFile) open() error {
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	r.file = f
	r.size = info.Size()
	return nil
}

func (r *rotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return 0, os.ErrClosed
	}
	if r.size+int64(len(p)) > r.maxBytes {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := r.file.Write(p)
	r.size += int64(n)
	return n, err
}

// rotate renames the active file to .1, shifts older files down, and deletes
// anything beyond maxFiles. Callers hold the mutex.
func (r *rotatingFile) rotate() error {
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			return err
		}
		r.file = nil
	}
	if r.maxFiles <= 1 {
		if err := os.Remove(r.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return r.open()
	}
	// Drop the oldest, then shift each rotated file one slot down.
	oldest := fmt.Sprintf("%s.%d", r.path, r.maxFiles-1)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		return err
	}
	for i := r.maxFiles - 2; i >= 1; i-- {
		from := fmt.Sprintf("%s.%d", r.path, i)
		to := fmt.Sprintf("%s.%d", r.path, i+1)
		if err := os.Rename(from, to); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Rename(r.path, r.path+".1"); err != nil && !os.IsNotExist(err) {
		return err
	}
	return r.open()
}

func (r *rotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}
