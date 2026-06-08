package room

import (
	"log/slog"
	"sync"

	"github.com/gokuljs/goSfu/pkg/config"
	"github.com/google/uuid"
)

type Manager struct {
	rooms     map[string]*Room
	mu        sync.RWMutex
	audioPath string
}

func NewManager() *Manager {
	return &Manager{
		rooms:     make(map[string]*Room),
		audioPath: config.DEFAULT_AUDIO_SAMPLE_FILE,
	}
}

func (m *Manager) Create() string {
	id := uuid.New().String()
	room := NewRoom(id, func(roomCloseId string) {
		m.Delete(roomCloseId)
	})
	m.mu.Lock()
	m.rooms[id] = room
	m.mu.Unlock()
	slog.Info("room created", "room", id)
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
	slog.Info("room deleted", "room", id)
}
