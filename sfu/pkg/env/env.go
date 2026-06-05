// Package env loads configuration from the process environment.
//
// Production (Railway, Kubernetes, etc.): set variables in the platform dashboard.
// They are injected into the process — no .env file on the server.
//
// Local development: copy .env.example to .env and fill in secrets.
// godotenv merges .env into the environment but never overwrites variables
// that are already set (12-factor: production config wins).
package env

import (
	"errors"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

// Load merges an optional .env file from the working directory into the
// process environment. Call once at the very start of main(), before
// NewConfig() or any plugin reads os.Getenv for API keys.
//
// Missing .env is normal in production and is not treated as an error.
func Load() {
	if err := godotenv.Load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		// godotenv may return a wrapped path error on some platforms
		if os.IsNotExist(err) {
			return
		}
		slog.Warn("env: could not load .env", "error", err)
		return
	}
	slog.Debug("env: loaded .env")
}
