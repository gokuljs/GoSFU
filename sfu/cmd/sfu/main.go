package main

import (
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/gokuljs/goSfu/pkg/config"
	"github.com/gokuljs/goSfu/pkg/env"
	"github.com/gokuljs/goSfu/pkg/logger"
	"github.com/gokuljs/goSfu/pkg/redisroom"
	"github.com/gokuljs/goSfu/pkg/room"
	"github.com/gokuljs/goSfu/pkg/server"
)

func main() {
	port := flag.Int("port", config.DEFAULT_PORT, "http server port")
	envName := flag.String("env", envOr("ENV", "local"), "environment (local|development|production)")
	loadTest := flag.Bool("load-test", false, "use stub AI providers (no external API calls)")
	flag.Parse()

	logger.Init(logger.EnvFromString(*envName))
	env.Load()

	sessionMax := parseDuration(envOr("SESSION_MAX_DURATION", "30m"), 30*time.Minute)
	nodeID := envOr("NODE_ID", "local")
	redisURL := os.Getenv("REDIS_URL")

	var redisStore *redisroom.Store
	redisMode := "fallback"
	if redisURL != "" {
		store, err := redisroom.NewStore(redisURL, nodeID)
		if err != nil {
			slog.Warn("redis unavailable, using in-memory fallback", "error", err)
		} else if store != nil {
			redisStore = store
			redisMode = "enabled"
			defer redisStore.Close()
		}
	}

	slog.Info("starting server",
		"port", *port,
		"env", *envName,
		"redis", redisMode,
		"node_id", nodeID,
		"session_max", sessionMax.String(),
		"load_test", *loadTest,
	)

	manager := room.NewManager(redisStore, sessionMax, *loadTest)
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

func parseDuration(raw string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return def
	}
	return d
}
