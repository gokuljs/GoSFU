package room

import (
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

type Manager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

func (m *Manager) Create() string {
	id := uuid.New().String()
	room := NewRoom(id)
	m.mu.Lock()
	m.rooms[id] = room
	m.mu.Unlock()
	slog.Info("room created", "roomId", id)
	return id
}

func (m *Manager) Get(id string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.rooms[id]
	return room, ok
}

func (m *Manager) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, id)
	slog.Info("room deleted", "roomId", id)
}
