package agent

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/pkg/agent/transport"
	"github.com/gokuljs/goSfu/plugins/llm"
	"github.com/gokuljs/goSfu/plugins/vad"
)

// Orchestrator is the single owner of conversation state. Everything runs on
// its one Run goroutine, so there is no shared mutable state across goroutines
// and therefore no locks: inbound audio, VAD events, STT results, LLM streaming
// and TTS playback are all sequenced here.
type Orchestrator struct {
	cfg       Config
	transport transport.Transport
	history   []llm.Message
}

func NewOrchestrator(cfg Config, t transport.Transport) *Orchestrator {
	o := &Orchestrator{cfg: cfg, transport: t}
	if p := strings.TrimSpace(cfg.Settings.SystemPrompt); p != "" {
		o.history = append(o.history, llm.Message{Role: llm.RoleSystem, Content: p})
	}
	return o
}

// Run is the conversation loop. It owns one STT session and one VAD instance
// for the lifetime of the call.
//
// Note (half-duplex for now): handleUtterance blocks the loop while the agent
// speaks, so inbound mic frames buffer and drop during agent speech. That is
// fine until barge-in (Phase 5), which will move handleUtterance onto its own
// cancellable goroutine.
func (o *Orchestrator) Run(ctx context.Context) {
	sttSess, err := o.cfg.Plugins.STT.NewSession(ctx)
	if err != nil {
		slog.Error("stt session init failed", "error", err)
		return
	}
	defer sttSess.Close()

	o.cfg.Plugins.VAD.Reset()
	defer o.cfg.Plugins.VAD.Close()

	sttRate := o.cfg.Plugins.STT.SampleRate()
	vadRate := o.cfg.Plugins.VAD.SampleRate()

	// Greet the user as soon as the loop starts (exercises the outbound path).
	if g := strings.TrimSpace(o.cfg.Settings.GreetingText); g != "" {
		o.speak(ctx, g)
	}

	in := o.transport.Inbound()
	results := sttSess.Results()

	for {
		select {
		case <-ctx.Done():
			return

		case frame, ok := <-in:
			if !ok {
				return
			}
			// VAD on a copy resampled to the model's rate (turn detection).
			vf := audio.Frame{
				Samples:    audio.Resample(frame.Samples, frame.SampleRate, vadRate),
				SampleRate: vadRate,
			}
			if events, verr := o.cfg.Plugins.VAD.Analyze(ctx, vf); verr == nil {
				for _, e := range events {
					switch e.Type {
					case vad.SpeechStart:
						slog.Debug("vad: speech start")
						// Phase 5: if o.cfg.Settings.BargeIn, cancel current speak() here.
					case vad.SpeechEnd:
						slog.Debug("vad: speech end")
					}
				}
			}
			// Feed STT at its native rate.
			sf := audio.Frame{
				Samples:    audio.Resample(frame.Samples, frame.SampleRate, sttRate),
				SampleRate: sttRate,
			}
			_ = sttSess.SendFrame(ctx, sf)

		case res, ok := <-results:
			if !ok {
				return
			}
			if res.IsFinal && strings.TrimSpace(res.Text) != "" {
				o.handleUtterance(ctx, res.Text)
			}
		}
	}
}

// handleUtterance turns one finalized user sentence into spoken audio:
// LLM stream → sentence chunks → TTS → speaker.
func (o *Orchestrator) handleUtterance(ctx context.Context, userText string) {
	slog.Info("user said", "text", userText)
	o.history = append(o.history, llm.Message{Role: llm.RoleUser, Content: userText})

	stream, err := o.cfg.Plugins.LLM.StreamCompletion(ctx, o.history)
	if err != nil {
		slog.Error("llm stream failed", "error", err)
		return
	}

	chunker := newSentenceChunker(o.cfg.Settings.MaxChunkChars)
	var full strings.Builder

	for chunk := range stream {
		if chunk.Err != nil {
			slog.Error("llm stream error", "error", chunk.Err)
			break
		}
		full.WriteString(chunk.Delta)
		// Speak each completed sentence immediately so audio starts before the
		// LLM has finished generating the whole reply.
		for _, sentence := range chunker.push(chunk.Delta) {
			if !o.speak(ctx, sentence) {
				return // context cancelled mid-reply
			}
		}
		if chunk.Done {
			break
		}
	}
	// Flush the trailing partial sentence (or punctuation-free remainder).
	for _, sentence := range chunker.flush() {
		if !o.speak(ctx, sentence) {
			return
		}
	}

	o.history = append(o.history, llm.Message{Role: llm.RoleAssistant, Content: full.String()})
}

// speak renders one text chunk to audio and pushes it to the transport.
// Returns false if the context was cancelled (caller should stop).
//
// Frequency handling lives here: TTS emits at its own native rate; a per-
// utterance StreamResampler converts to 48k without per-chunk drift, and a
// SampleBuffer reframes the continuous stream into 20ms frames.
func (o *Orchestrator) speak(ctx context.Context, text string) bool {
	slog.Info("agent speaking", "text", text)

	chunks, err := o.cfg.Plugins.TTS.Synthesize(ctx, text)
	if err != nil {
		slog.Error("tts failed", "error", err)
		return true // skip this sentence, keep the conversation alive
	}

	rs := audio.NewStreamResampler(o.cfg.Plugins.TTS.SampleRate(), audio.WebrtcSampleRate)
	reframe := audio.NewSampleBuffer(audio.WebrtcSampleRate)

	for chunk := range chunks {
		if chunk.Err != nil {
			slog.Error("tts chunk error", "error", chunk.Err)
			break
		}
		if len(chunk.Samples) > 0 {
			for _, f := range reframe.Push(rs.Process(chunk.Samples)) {
				if !o.send(ctx, f) {
					return false
				}
			}
		}
		if chunk.Done {
			break
		}
	}
	// Drain the resampler tail.
	for _, f := range reframe.Push(rs.Flush()) {
		if !o.send(ctx, f) {
			return false
		}
	}
	return true
}

func (o *Orchestrator) send(ctx context.Context, f audio.Frame) bool {
	select {
	case <-ctx.Done():
		return false
	default:
		_ = o.transport.Send(f)
		return true
	}
}
