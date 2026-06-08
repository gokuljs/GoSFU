package logger

import (
	"context"
	"log/slog"
	"strings"
)

// Pipeline event names — stable keys for Grafana/Loki queries.
const (
	EventAudioFirstFrame  = "pipeline.audio.first_frame"
	EventAudioLevel       = "pipeline.audio.level"
	EventVADSpeechStart   = "pipeline.vad.speech_start"
	EventVADSpeechEnd     = "pipeline.vad.speech_end"
	EventSTTFrameSent     = "pipeline.stt.frame_sent"
	EventSTTFrameFailed   = "pipeline.stt.frame_failed"
	EventSTTEmptyResult   = "pipeline.stt.empty_result"
	EventSTTResult        = "pipeline.stt.result"
	EventTurnReady        = "pipeline.turn.ready"
	EventLLMRequest       = "pipeline.llm.request"
	EventLLMFirstToken    = "pipeline.llm.first_token"
	EventLLMComplete      = "pipeline.llm.complete"
	EventTTSRequest       = "pipeline.tts.request"
	EventTTSFirstAudio    = "pipeline.tts.first_audio"
	EventPlayoutStarted   = "pipeline.playout.started"
	EventPlayoutDrained   = "pipeline.playout.drained"
	EventTurnComplete     = "pipeline.turn.complete"
	EventAudioFrameDrop   = "pipeline.audio.frame_drop"
	EventAgentStart       = "pipeline.agent.start"
	EventAgentStop        = "pipeline.agent.stop"
	EventSTTSessionFailed = "pipeline.stt.session_failed"
	EventLLMFailed        = "pipeline.llm.failed"
	EventTTSFailed        = "pipeline.tts.failed"
)

// Pipeline logs a voice-pipeline event with a stable event key and human-readable msg.
func Pipeline(level slog.Level, event, msg string, attrs ...any) {
	args := make([]any, 0, len(attrs)+2)
	args = append(args, "event", event)
	args = append(args, attrs...)
	slog.Log(context.Background(), level, msg, args...)
}

// WithSession returns a child logger that always includes room and turn.
func WithSession(room string, turn int) *slog.Logger {
	return slog.With("room", room, "turn", turn)
}

// Preview truncates text for log previews.
func Preview(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
