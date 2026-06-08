package room

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/gokuljs/goSfu/pkg/agent"
	"github.com/gokuljs/goSfu/pkg/agent/transport"
	"github.com/gokuljs/goSfu/pkg/config"
	"github.com/gokuljs/goSfu/pkg/roomstream"
	"github.com/gokuljs/goSfu/pkg/sfu"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type State string

const (
	StateActive  State = "active"
	StateClosed  State = "closed"
	StateWaiting State = "wait_for_user"
)

var (
	ErrRoomClosed = fmt.Errorf("room closed")
	ErrRoomFull   = fmt.Errorf("room full")
)

type Participant struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type Room struct {
	mu           sync.Mutex
	Id           string
	State        State
	Participants []Participant
	audioPath    string
	onClose      func(string)
	ctx          context.Context
	cancel       context.CancelFunc
	pc           *webrtc.PeerConnection
	agent        *agent.Agent
	stream       *roomstream.Hub
}
type JoinResult struct {
	Sdp           webrtc.SessionDescription `json:"sdp"`
	ParticipantId string                    `json:"participantId"`
	RoomId        string                    `json:"roomId"`
}

func NewRoom(id string, stream *roomstream.Hub, onClose func(string)) *Room {
	return &Room{
		Id:           id,
		State:        StateWaiting,
		Participants: []Participant{},
		audioPath:    config.DEFAULT_AUDIO_SAMPLE_FILE,
		onClose:      onClose,
		stream:       stream,
	}
}

func (r *Room) HandleJoin(offer webrtc.SessionDescription, systemPrompt string) (*JoinResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// check if the room is already full or already closed
	if r.State == StateClosed {
		return nil, ErrRoomClosed
	}
	if r.State == StateActive {
		return nil, ErrRoomFull
	}
	participantId := uuid.New().String()
	r.Participants = []Participant{{
		Id:     participantId,
		Name:   "",
		Active: true,
	}}
	r.stream.PublishEvent(r.Id, "session.participant.joined", "info", "Participant joined", map[string]any{
		"participant_id": participantId,
	})

	pc, err := sfu.CreatePeerConnectionWithInterceptors(config.STUN_SERVER)
	if err != nil {
		return nil, err
	}
	r.pc = pc

	tr, err := transport.NewWebrtc(pc, r.Id)
	if err != nil {
		r.stopSessionLocked()
		return nil, err
	}

	// Provider wiring lives in code, not env. Only secrets (API keys) and
	// machine-specific paths come from the environment, resolved inside each
	// plugin's factory.
	settings := agent.DefaultSettings()
	if prompt := strings.TrimSpace(systemPrompt); prompt != "" {
		settings.SystemPrompt = prompt
	}
	cfg, err := agent.NewConfig(agent.Options{
		LLMProvider: "openai",
		STTProvider: "deepgram",
		TTSProvider: "rime",
		VADProvider: "silero",
		Settings:    settings,
	})
	if err != nil {
		r.stopSessionLocked()
		return nil, err
	}
	cfg.RoomID = r.Id
	cfg.TranscriptPublisher = r.stream
	cfg.MetricsPublisher = r.stream

	sessionCtx, cancel := context.WithCancel(context.Background())
	r.ctx = sessionCtx
	r.cancel = cancel

	ag := agent.New(sessionCtx, tr, cfg)
	r.agent = ag

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		slog.Info("track received",
			"room", r.Id,
			"kind", track.Kind().String(),
			"codec", track.Codec().MimeType,
		)
		r.stream.PublishEvent(r.Id, "media.track.started", "info", "Track started", map[string]any{
			"kind":       track.Kind().String(),
			"codec":      track.Codec().MimeType,
			"clock_rate": track.Codec().ClockRate,
		})
		// Audio only — keep video out of the Opus decoder.
		if track.Kind() != webrtc.RTPCodecTypeAudio {
			go r.drainTrack(sessionCtx, track)
			return
		}
		tr.HandleRemoteTrack(sessionCtx, track)
	})

	var agentStarted sync.Once
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Info("pc state", "room", r.Id, "state", state.String())
		r.stream.PublishEvent(r.Id, "transport.peer_connection.state", "info", "Peer connection state changed", map[string]any{
			"state": state.String(),
		})
		if state == webrtc.PeerConnectionStateConnected {
			agentStarted.Do(func() { ag.Start() })
		}
		if state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed ||
			state == webrtc.PeerConnectionStateDisconnected {
			go r.StopSession()
		}
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		r.stopSessionLocked()
		return nil, err
	}

	// Job 7: create and set answer which need to be send back to browser and stuff
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		r.stopSessionLocked()
		return nil, err
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		r.stopSessionLocked()
		return nil, err
	}
	// basically waiting for all ice setup done
	<-gatherComplete
	r.State = StateActive
	slog.Info("room active", "room", r.Id, "participant", participantId)
	r.stream.PublishEvent(r.Id, "session.room.active", "info", "Room active", map[string]any{
		"participant_id": participantId,
	})
	return &JoinResult{
		Sdp:           *pc.LocalDescription(),
		ParticipantId: participantId,
		RoomId:        r.Id,
	}, nil
}

func (r *Room) drainTrack(ctx context.Context, track *webrtc.TrackRemote) {
	// 1500 is MTU size in general
	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if _, _, err := track.Read(buf); err != nil {
			r.stream.PublishEvent(r.Id, "media.track.stopped", "info", "Track stopped", map[string]any{
				"kind":  track.Kind().String(),
				"codec": track.Codec().MimeType,
				"error": err.Error(),
			})
			return
		}
	}
}

func (r *Room) StopSession() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.State == StateClosed {
		return
	}
	r.stopSessionLocked()
}

func (r *Room) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.State == StateClosed {
		return
	}
	id := r.Id
	r.State = StateClosed
	r.stopSessionLocked()
	slog.Info("room closed", "room", r.Id)
	r.stream.PublishEvent(r.Id, "session.room.closed", "info", "Room closed", nil)
	if r.onClose != nil {
		r.onClose(id)
	}
}

func (r *Room) stopSessionLocked() {
	hadSession := r.cancel != nil || r.agent != nil || r.pc != nil
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	r.ctx = nil
	if r.agent != nil {
		r.agent.Stop()
		r.agent = nil
	}
	if r.pc != nil {
		_ = r.pc.Close()
		r.pc = nil
	}
	for i := range r.Participants {
		if r.Participants[i].Active {
			hadSession = true
		}
		r.Participants[i].Active = false
	}
	if r.State != StateClosed {
		r.State = StateWaiting
		if hadSession {
			slog.Info("room session stopped", "room", r.Id)
			r.stream.PublishEvent(r.Id, "session.room.waiting", "info", "Room waiting for session", nil)
		}
	}
}
