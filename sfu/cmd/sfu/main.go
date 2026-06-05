package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/gokuljs/goSfu/pkg/config"
	"github.com/gokuljs/goSfu/pkg/env"
	"github.com/gokuljs/goSfu/pkg/logger"
	"github.com/gokuljs/goSfu/pkg/room"
	"github.com/gokuljs/goSfu/pkg/server"
)

func main() {
	// Merge optional .env before flags/plugins read os.Getenv.
	// Production injects env vars directly; .env is local-only.
	env.Load()

	port := flag.Int("port", config.DEFAULT_PORT, "http server port")
	envName := flag.String("env", envOr("ENV", "development"), "environment (development|production)")
	flag.Parse()
	logger.Init(logger.EnvFromString(*envName))
	slog.Info("starting server", "port", *port)
	manager := room.NewManager()
	srv := server.New(*port, manager)

	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
