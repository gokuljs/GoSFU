package roomquota

import (
	"errors"
	"strings"
	"sync"
	"time"
)

const DefaultLimit = 15

var ErrExhausted = errors.New("session quota exhausted")

type State struct {
	Room       string    `json:"room"`
	Limit      int       `json:"limit"`
	Used       int       `json:"used"`
	ActiveTurn int       `json:"activeTurn,omitempty"`
	Exhausted  bool      `json:"exhausted"`
	Message    string    `json:"message,omitempty"`
	TS         time.Time `json:"ts"`
}

type roomState struct {
	limit          int
	used           int
	activeTurn     int
	completedTurns map[int]struct{}
}

type Store struct {
	mu    sync.Mutex
	limit int
	rooms map[string]*roomState
}

func NewStore(limit int) *Store {
	if limit <= 0 {
		limit = DefaultLimit
	}
	return &Store{
		limit: limit,
		rooms: make(map[string]*roomState),
	}
}

func (s *Store) Reset(room string) State {
	s.mu.Lock()
	defer s.mu.Unlock()

	rs := &roomState{
		limit:          s.limit,
		completedTurns: make(map[int]struct{}),
	}
	s.rooms[room] = rs
	return snapshot(room, rs, "Session quota ready")
}

func (s *Store) Delete(room string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rooms, room)
}

func (s *Store) Get(room string) State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return snapshot(room, s.ensureLocked(room), "Session quota updated")
}

func (s *Store) IsExhausted(room string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureLocked(room).used >= s.ensureLocked(room).limit
}

func (s *Store) StartTurn(room string, turn int) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rs := s.ensureLocked(room)
	if rs.used >= rs.limit {
		rs.activeTurn = 0
		return snapshot(room, rs, exhaustedMessage()), ErrExhausted
	}
	if turn > 0 {
		rs.activeTurn = turn
	}
	return snapshot(room, rs, "Session quota turn started"), nil
}

func (s *Store) CompleteTurn(room string, turn int) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rs := s.ensureLocked(room)
	if turn <= 0 {
		return snapshot(room, rs, "Free greeting completed"), nil
	}
	if _, ok := rs.completedTurns[turn]; ok {
		return snapshot(room, rs, "Session quota already counted"), nil
	}
	if rs.used >= rs.limit {
		rs.activeTurn = 0
		return snapshot(room, rs, exhaustedMessage()), ErrExhausted
	}

	rs.completedTurns[turn] = struct{}{}
	rs.used++
	if rs.activeTurn == turn {
		rs.activeTurn = 0
	}
	if rs.used >= rs.limit {
		return snapshot(room, rs, exhaustedMessage()), ErrExhausted
	}
	return snapshot(room, rs, "Session quota turn completed"), nil
}

func (s *Store) ensureLocked(room string) *roomState {
	key := strings.TrimSpace(room)
	if key == "" {
		key = "unknown"
	}
	if rs := s.rooms[key]; rs != nil {
		return rs
	}
	rs := &roomState{
		limit:          s.limit,
		completedTurns: make(map[int]struct{}),
	}
	s.rooms[key] = rs
	return rs
}

func snapshot(room string, rs *roomState, message string) State {
	return State{
		Room:       room,
		Limit:      rs.limit,
		Used:       rs.used,
		ActiveTurn: rs.activeTurn,
		Exhausted:  rs.used >= rs.limit,
		Message:    message,
		TS:         time.Now().UTC(),
	}
}

func exhaustedMessage() string {
	return "You are out of usage quota. Further voice requests are blocked for this session."
}
