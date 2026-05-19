package server

import (
	"log/slog"
	"net/http"
	"strconv"
)

type Server struct {
	port    int
	sdpChan chan string
}

func HttpSdpServer(port int) chan string {
	sdpChan := make(chan string)
	srv := &Server{
		port:    port,
		sdpChan: sdpChan,
	}
	srv.setupRoutes()
	go func() {
		addr := ":" + strconv.Itoa(port)
		slog.Info("HTTP server listening", "addr", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			slog.Error("HTTP server crashed", "error", err)
		}
	}()
	return sdpChan
}
