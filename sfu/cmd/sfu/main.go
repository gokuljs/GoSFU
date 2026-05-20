package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/gokuljs/goSfu/pkg/config"
	"github.com/gokuljs/goSfu/pkg/logger"
	"github.com/gokuljs/goSfu/pkg/room"
	"github.com/gokuljs/goSfu/pkg/server"
)

func main() {
	port := flag.Int("port", config.DEFAULT_PORT, "http server port")
	env := flag.String("env", "development", "environment")
	flag.Parse()
	logger.Init(logger.EnvFromString(*env))
	slog.Info("starting server", "port", *port)

	manager := room.NewManager()
	srv := server.New(*port, manager)

	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
