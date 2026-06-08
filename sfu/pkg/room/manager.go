package room

import (
	"log/slog"
	"sync"

	"github.com/gokuljs/goSfu/pkg/config"
	"github.com/gokuljs/goSfu/pkg/logger"
	"github.com/gokuljs/goSfu/pkg/sessiondebug"
	"github.com/google/uuid"
)

type Manager struct {
	rooms     map[string]*Room
	mu        sync.RWMutex
	audioPath string
	debug     *sessiondebug.Hub
}

func NewManager() *Manager {
	debug := sessiondebug.NewHub()
	logger.SetPipelineSink(debug.PublishPipeline)
	return &Manager{
		rooms:     make(map[string]*Room),
		audioPath: config.DEFAULT_AUDIO_SAMPLE_FILE,
		debug:     debug,
	}
}

func (m *Manager) Create() string {
	id := uuid.New().String()
	room := NewRoom(id, m.debug, func(roomCloseId string) {
		m.Delete(roomCloseId)
	})
	m.mu.Lock()
	m.rooms[id] = room
	m.mu.Unlock()
	slog.Info("room created", "room", id)
	m.debug.PublishEvent(id, "session.room.created", "info", "Room created", nil)
	return id
}

func (m *Manager) Get(id string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.rooms[id]
	return room, ok
}

func (m *Manager) Debug() *sessiondebug.Hub {
	return m.debug
}

func (m *Manager) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, id)
	slog.Info("room deleted", "room", id)
	m.debug.PublishEvent(id, "session.room.deleted", "info", "Room deleted", nil)
}
