package room

import (
	"log/slog"
	"sync"

	"github.com/gokuljs/goSfu/pkg/config"
	"github.com/gokuljs/goSfu/pkg/logger"
	"github.com/gokuljs/goSfu/pkg/roomquota"
	"github.com/gokuljs/goSfu/pkg/roomstream"
	"github.com/google/uuid"
)

type Manager struct {
	rooms     map[string]*Room
	mu        sync.RWMutex
	audioPath string
	stream    *roomstream.Hub
	quota     *roomquota.Store
}

func NewManager() *Manager {
	stream := roomstream.NewHub()
	quota := roomquota.NewStore(roomquota.DefaultLimit)
	logger.SetPipelineSink(stream.PublishPipeline)
	return &Manager{
		rooms:     make(map[string]*Room),
		audioPath: config.DEFAULT_AUDIO_SAMPLE_FILE,
		stream:    stream,
		quota:     quota,
	}
}

func (m *Manager) Create() string {
	id := uuid.New().String()
	room := NewRoom(id, m.stream, m.quota, func(roomCloseId string) {
		m.Delete(roomCloseId)
	})
	m.mu.Lock()
	m.rooms[id] = room
	m.mu.Unlock()
	slog.Info("room created", "room", id)
	m.stream.PublishQuota(id, m.quota.Reset(id))
	m.stream.PublishEvent(id, "session.room.created", "info", "Room created", nil)
	return id
}

func (m *Manager) Get(id string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.rooms[id]
	return room, ok
}

func (m *Manager) Stream() *roomstream.Hub {
	return m.stream
}

func (m *Manager) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, id)
	slog.Info("room deleted", "room", id)
	m.quota.Delete(id)
	m.stream.PublishEvent(id, "session.room.deleted", "info", "Room deleted", nil)
}
