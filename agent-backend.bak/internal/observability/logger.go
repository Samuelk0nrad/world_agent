package observability

import (
	"log/slog"
	"os"
	"strings"
)

func NewLogger(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}

	if strings.EqualFold(strings.TrimSpace(format), "text") {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
