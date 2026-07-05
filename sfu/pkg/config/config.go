package config

import (
	"os"
	"strings"

	"github.com/pion/webrtc/v4"
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

func ICEServers() []webrtc.ICEServer {
	servers := []webrtc.ICEServer{
		{URLs: STUNServers()},
	}

	if turnURLs := splitURLs(os.Getenv("TURN_URLS")); len(turnURLs) > 0 {
		servers = append(servers, webrtc.ICEServer{
			URLs:       turnURLs,
			Username:   strings.TrimSpace(os.Getenv("TURN_USERNAME")),
			Credential: strings.TrimSpace(os.Getenv("TURN_CREDENTIAL")),
		})
	}

	return servers
}

func STUNServers() []string {
	urls := splitURLs(os.Getenv("STUN_URLS"))
	if len(urls) == 0 {
		return DefaultSTUNServers
	}
	return urls
}

func splitURLs(raw string) []string {
	parts := strings.Split(raw, ",")
	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		if url := strings.TrimSpace(part); url != "" {
			urls = append(urls, url)
		}
	}
	return urls
}
