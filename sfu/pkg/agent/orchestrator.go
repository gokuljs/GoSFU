package agent

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/pkg/agent/transport"
	"github.com/gokuljs/goSfu/pkg/logger"
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
	turn         int
	turnStart    time.Time // wall clock when current turn processing began

	responseCancel context.CancelFunc
	responseDone   <-chan responseResult

	sttFrames       uint64
	nextSTTLevelLog time.Time
}

type responseResult struct {
	turn          int
	assistantText string
	completed     bool
	interrupted   bool
}

func NewOrchestrator(cfg Config, t transport.Transport) *Orchestrator {
	o := &Orchestrator{cfg: cfg, transport: t, state: stateListening}
	if p := strings.TrimSpace(cfg.Settings.SystemPrompt); p != "" {
		o.history = append(o.history, llm.Message{Role: llm.RoleSystem, Content: p})
	}
	return o
}

func (o *Orchestrator) room() string { return o.cfg.RoomID }

// Run is the conversation loop. It owns one STT session and one VAD instance
// for the lifetime of the call and sequences turns: listen → respond → listen.
func (o *Orchestrator) Run(ctx context.Context) {
	sttSess, err := o.cfg.Plugins.STT.NewSession(ctx)
	if err != nil {
		logger.Pipeline(slog.LevelError, logger.EventSTTSessionFailed,
			"STT session init failed",
			"room", o.room(), "error", err,
		)
		return
	}
	defer sttSess.Close()

	o.cfg.Plugins.VAD.Reset()
	defer o.cfg.Plugins.VAD.Close()

	sttRate := o.cfg.Plugins.STT.SampleRate()
	vadRate := o.cfg.Plugins.VAD.SampleRate()

	in := o.transport.Inbound()
	results := sttSess.Results()

	// Greet the user as soon as the loop starts (also exercises the outbound path).
	if g := strings.TrimSpace(o.cfg.Settings.GreetingText); g != "" {
		o.startGreeting(ctx, g)
	}

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

		case res := <-o.responseDone:
			o.onResponseDone(res)
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
					if o.state == stateResponding {
						o.interruptResponse("vad_speech_start")
					}
					logger.Pipeline(slog.LevelInfo, logger.EventVADSpeechStart,
						"User started speaking",
						"room", o.room(), "turn", o.turn+1,
					)
				}
			case vad.SpeechEnd:
				if o.userSpeaking {
					o.userSpeaking = false
					logger.Pipeline(slog.LevelDebug, logger.EventVADSpeechEnd,
						"User stopped speaking",
						"room", o.room(), "turn", o.turn+1,
					)
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
	if err := sttSess.SendFrame(ctx, sf); err != nil {
		logger.Pipeline(slog.LevelWarn, logger.EventSTTFrameFailed,
			"STT frame send failed",
			"room", o.room(), "turn", o.turn+1,
			"sample_rate", sf.SampleRate,
			"samples", len(sf.Samples),
			"error", err,
		)
		return
	}
	o.sttFrames++
	now := time.Now()
	if o.sttFrames == 1 || now.After(o.nextSTTLevelLog) {
		o.nextSTTLevelLog = now.Add(time.Second)
		logger.Pipeline(slog.LevelDebug, logger.EventSTTFrameSent,
			"STT frame sent",
			"room", o.room(), "turn", o.turn+1,
			"frame_count", o.sttFrames,
			"sample_rate", sf.SampleRate,
			"samples", len(sf.Samples),
			"rms", int(audio.RMS(sf.Samples)),
		)
	}
}

// onTranscript buffers finalized transcript segments for the current turn.
// Interim results are also useful as a barge-in signal while the agent is
// speaking, because they arrive even when VAD misses the start of speech.
func (o *Orchestrator) onTranscript(ctx context.Context, res stt.Result) {
	text := strings.TrimSpace(res.Text)
	if text == "" {
		return
	}
	if o.state == stateResponding {
		reason := "stt_interim"
		if res.IsFinal {
			reason = "stt_final"
		}
		o.interruptResponse(reason)
	}
	if !res.IsFinal {
		return
	}
	logger.Pipeline(slog.LevelDebug, logger.EventSTTResult,
		"STT transcript received",
		"room", o.room(), "turn", o.turn+1,
		"is_final", res.IsFinal,
		"confidence", res.Confidence,
		"text_len", len(text),
		"text_preview", logger.Preview(text, 80),
	)
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
	if o.state == stateResponding {
		return
	}
	text := strings.TrimSpace(o.pending.String())
	o.pending.Reset()
	if text == "" {
		return
	}

	o.turn++
	logger.Pipeline(slog.LevelInfo, logger.EventTurnReady,
		"Transcript ready — sending to LLM",
		"room", o.room(), "turn", o.turn,
		"text", text, "text_len", len(text),
	)
	o.startResponse(ctx, text)
}

func (o *Orchestrator) startResponse(ctx context.Context, userText string) {
	o.turnStart = time.Now()
	o.setState(stateResponding)

	o.history = append(o.history, llm.Message{Role: llm.RoleUser, Content: userText})
	history := append([]llm.Message(nil), o.history...)
	turn := o.turn

	respCtx, cancel := context.WithCancel(ctx)
	done := make(chan responseResult, 1)
	o.responseCancel = cancel
	o.responseDone = done

	go func() {
		assistantText, completed := o.runResponse(respCtx, turn, o.turnStart, history, userText)
		done <- responseResult{
			turn:          turn,
			assistantText: assistantText,
			completed:     completed,
			interrupted:   respCtx.Err() != nil && !completed,
		}
	}()
}

func (o *Orchestrator) startGreeting(ctx context.Context, text string) {
	o.turnStart = time.Now()
	o.setState(stateResponding)

	respCtx, cancel := context.WithCancel(ctx)
	done := make(chan responseResult, 1)
	o.responseCancel = cancel
	o.responseDone = done

	go func() {
		completed := o.speak(respCtx, 0, text)
		if completed {
			completed = o.transport.WaitForPlayout(respCtx) == nil
		}
		done <- responseResult{
			turn:        0,
			completed:   completed,
			interrupted: respCtx.Err() != nil && !completed,
		}
	}()
}

// runResponse runs one full turn: user text → LLM stream → sentence chunks
// → TTS → speaker. It runs off the main orchestrator loop so inbound VAD can
// keep detecting barge-in while the agent is speaking.
func (o *Orchestrator) runResponse(ctx context.Context, turn int, started time.Time, history []llm.Message, userText string) (string, bool) {

	logger.Pipeline(slog.LevelInfo, logger.EventLLMRequest,
		"Sending transcript to LLM",
		"room", o.room(), "turn", turn,
		"text", userText,
	)

	llmStart := time.Now()
	stream, err := o.cfg.Plugins.LLM.StreamCompletion(ctx, history)
	if err != nil {
		if ctx.Err() != nil {
			return "", false
		}
		logger.Pipeline(slog.LevelError, logger.EventLLMFailed,
			"LLM stream failed",
			"room", o.room(), "turn", turn, "error", err,
		)
		return "", false
	}

	chunker := newSentenceChunker(o.cfg.Settings.MaxChunkChars)
	var full strings.Builder
	var llmFirstToken bool

	for chunk := range stream {
		if chunk.Err != nil {
			if ctx.Err() != nil {
				return "", false
			}
			logger.Pipeline(slog.LevelError, logger.EventLLMFailed,
				"LLM stream error",
				"room", o.room(), "turn", turn, "error", chunk.Err,
			)
			break
		}
		if !llmFirstToken && chunk.Delta != "" {
			llmFirstToken = true
			logger.Pipeline(slog.LevelInfo, logger.EventLLMFirstToken,
				"LLM first token",
				"room", o.room(), "turn", turn,
				"ttfb_ms", time.Since(llmStart).Milliseconds(),
			)
		}
		full.WriteString(chunk.Delta)
		// Speak each completed sentence immediately so audio starts before the
		// LLM has finished generating the whole reply.
		for _, sentence := range chunker.push(chunk.Delta) {
			if !o.speak(ctx, turn, sentence) {
				return "", false // context cancelled mid-reply
			}
		}
		if chunk.Done {
			break
		}
	}
	// Flush the trailing partial sentence (or punctuation-free remainder).
	for _, sentence := range chunker.flush() {
		if !o.speak(ctx, turn, sentence) {
			return "", false
		}
	}

	logger.Pipeline(slog.LevelInfo, logger.EventLLMComplete,
		"LLM response complete",
		"room", o.room(), "turn", turn,
		"text_len", full.Len(),
		"duration_ms", time.Since(llmStart).Milliseconds(),
	)

	// Half-duplex: only hand the turn back to listening once the user has
	// actually heard the whole reply, not just when it was queued. speak()
	// returns after frames are buffered; the audio is still draining through the
	// pacer here.
	if err := o.transport.WaitForPlayout(ctx); err != nil {
		return "", false
	}

	logger.Pipeline(slog.LevelInfo, logger.EventTurnComplete,
		"Turn complete — reply finished playing",
		"room", o.room(), "turn", turn,
		"e2e_ms", time.Since(started).Milliseconds(),
	)

	return full.String(), true
}

// speak renders one text chunk to audio and pushes it to the transport.
// Returns false if the context was cancelled (caller should stop).
//
// Frequency handling lives here: TTS emits at its own native rate; a per-
// utterance StreamResampler converts to 48k without per-chunk drift, and a
// SampleBuffer reframes the continuous stream into 20ms frames.
func (o *Orchestrator) speak(ctx context.Context, turn int, text string) bool {
	logger.Pipeline(slog.LevelInfo, logger.EventTTSRequest,
		"TTS synthesizing sentence",
		"room", o.room(), "turn", turn,
		"text_preview", logger.Preview(text, 80),
	)

	ttsStart := time.Now()
	chunks, err := o.cfg.Plugins.TTS.Synthesize(ctx, text)
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		logger.Pipeline(slog.LevelError, logger.EventTTSFailed,
			"TTS failed",
			"room", o.room(), "turn", turn, "error", err,
		)
		return true // skip this sentence, keep the conversation alive
	}

	rs := audio.NewStreamResampler(o.cfg.Plugins.TTS.SampleRate(), audio.WebrtcSampleRate)
	reframe := audio.NewSampleBuffer(audio.WebrtcSampleRate)

	var ttsFirstAudio bool

	for chunk := range chunks {
		if chunk.Err != nil {
			if ctx.Err() != nil {
				return false
			}
			logger.Pipeline(slog.LevelError, logger.EventTTSFailed,
				"TTS chunk error",
				"room", o.room(), "turn", turn, "error", chunk.Err,
			)
			break
		}
		if len(chunk.Samples) > 0 {
			if !ttsFirstAudio {
				ttsFirstAudio = true
				logger.Pipeline(slog.LevelInfo, logger.EventTTSFirstAudio,
					"TTS first audio",
					"room", o.room(), "turn", turn,
					"ttfb_ms", time.Since(ttsStart).Milliseconds(),
				)
			}
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
		slog.Debug("state change", "room", o.room(), "from", o.state, "to", s)
		o.state = s
	}
}

func (o *Orchestrator) interruptResponse(reason string) {
	if o.responseCancel == nil {
		return
	}
	logger.Pipeline(slog.LevelInfo, logger.EventBargeIn,
		"User interrupted agent speech",
		"room", o.room(), "turn", o.turn,
		"reason", reason,
	)
	o.responseCancel()
	o.responseCancel = nil
	o.transport.ClearPlayout()
	o.setState(stateListening)
}

func (o *Orchestrator) onResponseDone(res responseResult) {
	if res.interrupted {
		logger.Pipeline(slog.LevelDebug, logger.EventBargeIn,
			"Interrupted response worker stopped",
			"room", o.room(), "turn", res.turn,
		)
	}
	if res.completed && res.turn == o.turn && strings.TrimSpace(res.assistantText) != "" {
		o.history = append(o.history, llm.Message{Role: llm.RoleAssistant, Content: res.assistantText})
	}
	if o.responseCancel != nil && res.turn == o.turn {
		o.responseCancel = nil
	}
	if res.turn == o.turn && o.state == stateResponding {
		o.setState(stateListening)
	}
	o.responseDone = nil
}

func (o *Orchestrator) send(ctx context.Context, f audio.Frame) bool {
	// Send is backpressured: it blocks while the playout buffer is full, which
	// paces TTS production to realtime instead of dropping frames.
	return o.transport.Send(ctx, f) == nil
}
