package server

import (
	"log/slog"
	"net/http"
	"strings"
)

func roomFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "room" {
		return parts[1]
	}
	return ""
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		room := roomFromPath(r.URL.Path)
		if room != "" {
			slog.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"room", room,
			)
		} else {
			slog.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
			)
		}
		next.ServeHTTP(w, r)
	})
}
