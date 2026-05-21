package room

import (
	"context"
	"fmt"
	"sync"

	"github.com/gokuljs/goSfu/pkg/config"
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
}
type JoinResult struct {
	Sdp           webrtc.SessionDescription `json:"sdp"`
	ParticipantId string                    `json:"participantId"`
	RoomId        string                    `json:"roomId"`
}

func NewRoom(id string, onClose func(string)) *Room {
	ctx, cancel := context.WithCancel(context.Background())
	return &Room{
		Id:           id,
		State:        StateWaiting,
		Participants: []Participant{},
		audioPath:    config.DEFAULT_AUDIO_SAMPLE_FILE,
		ctx:          ctx,
		cancel:       cancel,
		onClose:      onClose,
	}
}

func (r *Room) ReserveUser() error {
	if r.State == StateClosed {
		return ErrRoomClosed
	}
	if r.State == StateActive {
		return ErrRoomFull
	}
	r.State = StateActive
	return nil
}
