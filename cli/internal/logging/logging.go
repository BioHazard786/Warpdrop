package logging

import (
	"log/slog"
	"os"
)

func Init() {
	level := slog.LevelError // default: production only shows errors

	if l, ok := os.LookupEnv("LOG_LEVEL"); ok {
		switch l {
		case "dev", "development", "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn", "warning":
			level = slog.LevelWarn
		case "error", "production", "prod":
			level = slog.LevelError
		}
	}

	logger := slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		}),
	)
	slog.SetDefault(logger)
}
