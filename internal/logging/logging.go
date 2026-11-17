// Package logging configures the process-wide slog logger from FINANCER_LOG_*
// settings. The default is human-readable text on stderr; setting a file path
// appends there instead (useful when the process is not run under Docker).
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
		f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("opening log file: %w", err)
		}
		w = f
		closeFn = func() { _ = f.Close() }
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
