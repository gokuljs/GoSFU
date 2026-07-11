package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	baseURL := flag.String("base-url", envOr("SFU_URL", "http://localhost:8080"), "SFU base URL")
	clients := flag.Int("clients", 1, "number of concurrent headless clients (one room each)")
	duration := flag.Duration("duration", 30*time.Second, "how long each client stays connected")
	turns := flag.Int("turns", 0, "wait for N final agent transcript turns (0 = duration only)")
	stagger := flag.Duration("stagger", 100*time.Millisecond, "delay between spawning clients")
	wavPath := flag.String("wav", "", "optional mono 16-bit PCM WAV file for mic audio (default: 440 Hz sine)")
	systemPrompt := flag.String("system-prompt", "", "optional system prompt sent on join")
	waitServer := flag.Duration("wait-server", 10*time.Second, "max time to wait for SFU before starting")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if *clients <= 0 {
		fmt.Fprintln(os.Stderr, "--clients must be >= 1")
		os.Exit(2)
	}

	if err := waitForServer(*baseURL, *waitServer); err != nil {
		slog.Error("server unavailable", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("starting load test",
		"base_url", *baseURL,
		"clients", *clients,
		"duration", duration.String(),
		"turns", *turns,
		"stagger", stagger.String(),
		"wav", *wavPath,
	)

	results := make(chan sessionResult, *clients)
	var wg sync.WaitGroup

	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			if clientID > 0 && *stagger > 0 {
				select {
				case <-time.After(time.Duration(clientID) * *stagger):
				case <-ctx.Done():
					results <- sessionResult{clientID: clientID, err: ctx.Err()}
					return
				}
			}
			results <- runSession(ctx, sessionConfig{
				clientID:     clientID,
				baseURL:      *baseURL,
				wavPath:      *wavPath,
				systemPrompt: *systemPrompt,
				duration:     *duration,
				turns:        *turns,
			})
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var (
		okCount    int
		failCount  int
		totalTurns int
	)
	for result := range results {
		if result.err != nil {
			failCount++
			slog.Error("client failed",
				"client", result.clientID,
				"room", result.roomID,
				"error", result.err,
			)
			continue
		}
		okCount++
		totalTurns += result.agentTurns
	}

	slog.Info("load test complete",
		"ok", okCount,
		"failed", failCount,
		"agent_turns", totalTurns,
	)

	if failCount > 0 {
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
