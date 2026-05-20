package room

import "fmt"

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
	Id           string
	State        State
	Participants []Participant
}

func NewRoom(id string) *Room {
	return &Room{
		Id:           id,
		State:        StateWaiting,
		Participants: []Participant{},
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
