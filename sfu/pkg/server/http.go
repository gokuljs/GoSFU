package server

import (
	"io"
	"log/slog"
	"net/http"
)

func (s *Server) setupRoutes() {
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodOptions {
			setCORS(res)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		setCORS(res)

		body, err := io.ReadAll(req.Body)
		if err != nil {
			slog.Error("failed to read request body", "error", err)
			http.Error(res, "bad request", http.StatusBadRequest)
			return
		}

		slog.Debug("received SDP offer", "size", len(body), "remote", req.RemoteAddr)
		s.sdpChan <- string(body)

		res.Header().Set("Content-Type", "text/plain")
		res.Write([]byte("done"))
	})
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
