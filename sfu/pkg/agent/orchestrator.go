package agent

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/pkg/agent/transport"
	"github.com/gokuljs/goSfu/plugins/llm"
	"github.com/gokuljs/goSfu/plugins/stt"
	"github.com/gokuljs/goSfu/plugins/vad"
)

// convState is the conversation turn state. The agent is half-duplex for now:
// it either listens to the user or produces a reply, never both. Barge-in
// (interrupting the reply when the user speaks) is Phase 5.
type convState int

const (
	stateListening  convState = iota // receiving user audio, building a transcript
	stateResponding                  // LLM + TTS are producing the agent's reply
)

func (s convState) String() string {
	switch s {
	case stateListening:
		return "listening"
	case stateResponding:
		return "responding"
	default:
		return "unknown"
	}
}

// Orchestrator is the single owner of conversation state. Everything runs on
// its one Run goroutine, so there is no shared mutable state across goroutines
// and therefore no locks: inbound audio, VAD events, STT results, LLM streaming
// and TTS playback are all sequenced here.
type Orchestrator struct {
	cfg       Config
	transport transport.Transport
	history   []llm.Message

	state        convState
	userSpeaking bool            // VAD: is the user currently talking
	pending      strings.Builder // finalized STT text for the in-progress turn
}

func NewOrchestrator(cfg Config, t transport.Transport) *Orchestrator {
	o := &Orchestrator{cfg: cfg, transport: t, state: stateListening}
	if p := strings.TrimSpace(cfg.Settings.SystemPrompt); p != "" {
		o.history = append(o.history, llm.Message{Role: llm.RoleSystem, Content: p})
	}
	return o
}

// Run is the conversation loop. It owns one STT session and one VAD instance
// for the lifetime of the call and sequences turns: listen → respond → listen.
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

	// Greet the user as soon as the loop starts (also exercises the outbound path).
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
			o.onAudio(ctx, sttSess, frame, sttRate, vadRate)

		case res, ok := <-results:
			if !ok {
				return
			}
			o.onTranscript(ctx, res)
		}
	}
}

// onAudio forks one inbound frame to VAD (turn detection) and STT (transcription).
func (o *Orchestrator) onAudio(ctx context.Context, sttSess stt.Session, frame audio.Frame, sttRate, vadRate int) {
	// VAD on a copy resampled to the model's rate.
	vf := audio.Frame{
		Samples:    audio.Resample(frame.Samples, frame.SampleRate, vadRate),
		SampleRate: vadRate,
	}
	if events, err := o.cfg.Plugins.VAD.Analyze(ctx, vf); err == nil {
		for _, e := range events {
			switch e.Type {
			case vad.SpeechStart:
				if !o.userSpeaking {
					o.userSpeaking = true
					slog.Debug("turn: user started speaking")
				}
			case vad.SpeechEnd:
				if o.userSpeaking {
					o.userSpeaking = false
					slog.Debug("turn: user stopped speaking")
					o.maybeEndTurn(ctx)
				}
			}
		}
	}

	// Feed STT at its native rate.
	sf := audio.Frame{
		Samples:    audio.Resample(frame.Samples, frame.SampleRate, sttRate),
		SampleRate: sttRate,
	}
	_ = sttSess.SendFrame(ctx, sf)
}

// onTranscript buffers finalized transcript segments for the current turn.
// Interim (partial) results are ignored for now; a UI could surface them later.
func (o *Orchestrator) onTranscript(ctx context.Context, res stt.Result) {
	if !res.IsFinal {
		return
	}
	text := strings.TrimSpace(res.Text)
	if text == "" {
		return
	}
	if o.pending.Len() > 0 {
		o.pending.WriteByte(' ')
	}
	o.pending.WriteString(text)
	o.maybeEndTurn(ctx)
}

// maybeEndTurn fires a turn when the user has stopped speaking AND we have
// transcript. It is called from both the VAD path and the STT path so it works
// regardless of which signal arrives last:
//   - final-then-SpeechEnd: append buffers (still speaking), SpeechEnd ends it
//   - SpeechEnd-then-final: SpeechEnd sees empty buffer, the final ends it
func (o *Orchestrator) maybeEndTurn(ctx context.Context) {
	if o.userSpeaking || o.pending.Len() == 0 {
		return
	}
	text := strings.TrimSpace(o.pending.String())
	o.pending.Reset()
	if text == "" {
		return
	}
	o.handleUserText(ctx, text)
}

// handleUserText runs one full turn: user text → LLM stream → sentence chunks
// → TTS → speaker. It blocks the Run loop until the reply finishes (half-duplex);
// Phase 5 will move this onto a cancellable goroutine for barge-in.
func (o *Orchestrator) handleUserText(ctx context.Context, userText string) {
	slog.Info("user said", "text", userText)
	o.setState(stateResponding)
	defer o.setState(stateListening)

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

	// Half-duplex: only hand the turn back to listening once the user has
	// actually heard the whole reply, not just when it was queued. speak()
	// returns after frames are buffered; the audio is still draining through the
	// pacer here.
	_ = o.transport.WaitForPlayout(ctx)

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

func (o *Orchestrator) setState(s convState) {
	if o.state != s {
		slog.Debug("state change", "from", o.state, "to", s)
		o.state = s
	}
}

func (o *Orchestrator) send(ctx context.Context, f audio.Frame) bool {
	// Send is backpressured: it blocks while the playout buffer is full, which
	// paces TTS production to realtime instead of dropping frames.
	return o.transport.Send(ctx, f) == nil
}
