package logging

import (
	"log/slog"
	"os"
	"strings"
)

const EnvLogLevel = "MONOLIFT_LOG_LEVEL"

// ConfigureFromEnv installs a process-wide slog handler when MONOLIFT_LOG_LEVEL
// is set. Libraries keep using slog.Default(), so tests and CLIs can opt in
// without passing logger handles through every compiler call.
func ConfigureFromEnv() {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(EnvLogLevel)))
	if raw == "" {
		if os.Getenv("MONOLIFT_E2E_STAGE_LOG") != "1" {
			return
		}
		raw = "debug"
	}
	level, ok := parseLevel(raw)
	if !ok {
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

func parseLevel(raw string) (slog.Level, bool) {
	switch raw {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}
