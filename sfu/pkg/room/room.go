package room

type State string

const (
	StateActive  State = "active"
	StateClosed  State = "closed"
	StateWaiting State = "wait_for_user"
)

type Room struct {
	Id    string
	State State
}

func NewRoom(id string) *Room {
	return &Room{
		Id:    id,
		State: StateWaiting,
	}
}
