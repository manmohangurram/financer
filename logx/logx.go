// Package logx wraps the app's logger so the underlying implementation (today
// log/slog) can be swapped in one file. It also builds scoped loggers: bind
// request attributes once with Request()/With() and every log line from that
// request carries them automatically.
package logx

import (
	"log/slog"
	"os"
	"strings"
)

// Logger is the app's logger handle. Swap the backing implementation in Init
// and this type's methods when migrating to another library — call sites keep
// using logx.
type Logger struct {
	l *slog.Logger
}

func (l Logger) Debug(msg string, args ...any) { l.l.Debug(msg, args...) }
func (l Logger) Info(msg string, args ...any)  { l.l.Info(msg, args...) }
func (l Logger) Warn(msg string, args ...any)  { l.l.Warn(msg, args...) }
func (l Logger) Error(msg string, args ...any) { l.l.Error(msg, args...) }

// root is the app-wide logger. Reassigned in Init.
var root = slog.Default()

// Init configures the log level from FINANCER_LOG_LEVEL
// (debug | info | warn | error, default info). Call once at startup.
func Init() {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("FINANCER_LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	root = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(root)
}

// Request returns a logger scoped to one HTTP request. Build it once at the
// top of a handler; every line then carries method, path, and (when set) user.
func Request(method, path, user string) Logger {
	attrs := []any{"method", method, "path", path}
	if user != "" {
		attrs = append(attrs, "user", user)
	}
	return Logger{l: root.With(attrs...)}
}

// With returns a logger bound to attrs; every subsequent log line carries
// them. Useful for long-running methods that want shared context.
func With(attrs ...any) Logger {
	return Logger{l: root.With(attrs...)}
}

// Package-level helpers for logs without request context.
func Debug(msg string, args ...any) { root.Debug(msg, args...) }
func Info(msg string, args ...any)  { root.Info(msg, args...) }
func Warn(msg string, args ...any)  { root.Warn(msg, args...) }
func Error(msg string, args ...any) { root.Error(msg, args...) }
