package logger

import (
	"log/slog"
	"os"
)

type Env string

const (
	Dev  Env = "development"
	Prod Env = "production"
)

// Init configures the global slog logger based on the environment.
// Both modes output JSON (compatible with Grafana/Loki/Datadog).
// Dev mode: Debug level, source on all logs.
// Prod mode: Info level, source on all logs (needed for debugging in production).
func Init(env Env) *slog.Logger {
	var handler slog.Handler

	switch env {
	case Prod:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: true,
		})
	default:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		})
	}

	log := slog.New(handler)
	slog.SetDefault(log)
	return log
}

// EnvFromString parses a string into an Env value.
func EnvFromString(s string) Env {
	switch s {
	case "production", "prod":
		return Prod
	default:
		return Dev
	}
}
