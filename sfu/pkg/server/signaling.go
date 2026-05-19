package server

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"

	"github.com/pion/webrtc/v4"
)

// Decode decodes a base64-encoded JSON string into a SessionDescription.
func Decode(in string, obj *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		slog.Error("failed to base64 decode SDP", "error", err)
		panic(err)
	}
	if err = json.Unmarshal(b, obj); err != nil {
		slog.Error("failed to JSON unmarshal SDP", "error", err)
		panic(err)
	}
}

// Encode encodes a SessionDescription to a base64 JSON string.
func Encode(obj *webrtc.SessionDescription) string {
	b, err := json.Marshal(obj)
	if err != nil {
		slog.Error("failed to JSON marshal SDP", "error", err)
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}
