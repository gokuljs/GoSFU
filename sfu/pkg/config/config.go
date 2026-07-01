package config

import (
	"os"
	"strings"
)

var DefaultSTUNServers = []string{
	"stun:stun.l.google.com:19302",
	"stun:stun1.l.google.com:19302",
	"stun:stun.cloudflare.com:3478",
}

const (
	DEFAULT_PORT              = 8080
	DEFAULT_AUDIO_SAMPLE_FILE = "assets/sample.ogg"
)

func STUNServers() []string {
	raw := os.Getenv("STUN_URLS")
	if raw == "" {
		return DefaultSTUNServers
	}

	parts := strings.Split(raw, ",")
	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		if url := strings.TrimSpace(part); url != "" {
			urls = append(urls, url)
		}
	}
	if len(urls) == 0 {
		return DefaultSTUNServers
	}
	return urls
}
