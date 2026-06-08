package room

import (
	"context"
	"fmt"
	"log/slog"
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
	ctx, cancel := context.WithCancel(context.Background())
	return &Room{
		Id:           id,
		State:        StateWaiting,
		Participants: []Participant{},
		audioPath:    config.DEFAULT_AUDIO_SAMPLE_FILE,
		ctx:          ctx,
		cancel:       cancel,
		onClose:      onClose,
		stream:       stream,
	}
}

func (r *Room) HandleJoin(offer webrtc.SessionDescription) (*JoinResult, error) {
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
	r.Participants = append(r.Participants, Participant{
		Id:     participantId,
		Name:   "",
		Active: true,
	})
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
		r.cleanupLocked()
		return nil, err
	}

	// Provider wiring lives in code, not env. Only secrets (API keys) and
	// machine-specific paths come from the environment, resolved inside each
	// plugin's factory.
	cfg, err := agent.NewConfig(agent.Options{
		LLMProvider: "openai",
		STTProvider: "deepgram",
		TTSProvider: "rime",
		VADProvider: "silero",
	})
	if err != nil {
		r.cleanupLocked()
		return nil, err
	}
	cfg.RoomID = r.Id
	cfg.TranscriptPublisher = r.stream
	cfg.MetricsPublisher = r.stream

	ag := agent.New(r.ctx, tr, cfg)
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
			go r.drainTrack(track)
			return
		}
		tr.HandleRemoteTrack(r.ctx, track)
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
			r.Close()
		}
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		r.cleanupLocked()
		return nil, err
	}

	// Job 7: create and set answer which need to be send back to browser and stuff
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		r.cleanupLocked()
		return nil, err
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		r.cleanupLocked()
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

func (r *Room) drainTrack(track *webrtc.TrackRemote) {
	// 1500 is MTU size in general
	buf := make([]byte, 1500)
	for {
		select {
		case <-r.ctx.Done():
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

func (r *Room) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.State == StateClosed {
		return
	}
	id := r.Id
	r.cleanupLocked()
	if r.onClose != nil {
		r.onClose(id)
	}
}

func (r *Room) cleanupLocked() {
	r.State = StateClosed
	r.cancel()
	if r.agent != nil {
		r.agent.Stop()
		r.agent = nil
	}
	if r.pc != nil {
		_ = r.pc.Close()
		r.pc = nil
	}
	for i := range r.Participants {
		r.Participants[i].Active = false
	}
	slog.Info("room closed", "room", r.Id)
	r.stream.PublishEvent(r.Id, "session.room.closed", "info", "Room closed", nil)
}
