package logger

import (
	"log/slog"
	"os"
)

type Env string

const (
	Local Env = "local"
	Dev   Env = "development"
	Prod  Env = "production"
)

// Init configures the global slog logger based on the environment.
//
//   - local: colorful human-readable text (default for local dev)
//   - development: JSON, Debug level, source lines (Grafana/Loki)
//   - production: JSON, Info level, source lines (Grafana/Loki)
func Init(env Env) *slog.Logger {
	var handler slog.Handler

	switch env {
	case Prod:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: true,
		})
	case Dev:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		})
	default:
		handler = NewColorTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
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
	case "development", "dev":
		return Dev
	case "local", "":
		return Local
	default:
		return Local
	}
}
