package room

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gokuljs/goSfu/pkg/config"
	"github.com/gokuljs/goSfu/pkg/logger"
	"github.com/gokuljs/goSfu/pkg/redisroom"
	"github.com/gokuljs/goSfu/pkg/roomstream"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Manager struct {
	rooms      map[string]*Room
	mu         sync.RWMutex
	audioPath  string
	stream     *roomstream.Hub
	redis      *redisroom.Store
	sessionMax time.Duration
}

func NewManager(redisStore *redisroom.Store, sessionMax time.Duration) *Manager {
	if sessionMax <= 0 {
		sessionMax = 30 * time.Minute
	}
	var rdb *redis.Client
	if redisStore != nil {
		rdb = redisStore.Client()
	}
	stream := roomstream.NewHub(rdb)
	logger.SetPipelineSink(stream.PublishPipeline)
	return &Manager{
		rooms:      make(map[string]*Room),
		audioPath:  config.DEFAULT_AUDIO_SAMPLE_FILE,
		stream:     stream,
		redis:      redisStore,
		sessionMax: sessionMax,
	}
}

func (m *Manager) Create() string {
	id := uuid.New().String()
	room := NewRoom(id, m.stream, m.sessionMax, func(roomCloseId string) {
		m.Delete(roomCloseId)
	}, m.onActivity, m.onWaiting)
	m.mu.Lock()
	m.rooms[id] = room
	m.mu.Unlock()

	if m.redis != nil && m.redis.Enabled() {
		if err := m.redis.Register(context.Background(), id, string(StateWaiting)); err != nil {
			slog.Warn("redis room register failed", "room", id, "error", err)
		}
	}

	slog.Info("room created", "room", id)
	m.stream.PublishEvent(id, "session.room.created", "info", "Room created", nil)
	return id
}

func (m *Manager) Get(id string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.rooms[id]
	return room, ok
}

func (m *Manager) Exists(id string) bool {
	if _, ok := m.Get(id); ok {
		return true
	}
	if m.redis == nil || !m.redis.Enabled() {
		return false
	}
	ok, err := m.redis.Exists(context.Background(), id)
	if err != nil {
		slog.Warn("redis room exists check failed", "room", id, "error", err)
		return false
	}
	return ok
}

func (m *Manager) Stream() *roomstream.Hub {
	return m.stream
}

func (m *Manager) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rooms[id]; !ok {
		return
	}
	delete(m.rooms, id)
	slog.Info("room deleted", "room", id)
	m.stream.ClearRoom(id)
	if m.redis != nil && m.redis.Enabled() {
		if err := m.redis.Delete(context.Background(), id); err != nil {
			slog.Warn("redis room delete failed", "room", id, "error", err)
		}
	}
	m.stream.PublishEvent(id, "session.room.deleted", "info", "Room deleted", nil)
}

func (m *Manager) onActivity(roomID string) {
	if m.redis == nil || !m.redis.Enabled() {
		return
	}
	ctx := context.Background()
	if err := m.redis.Refresh(ctx, roomID); err != nil {
		slog.Warn("redis room refresh failed", "room", roomID, "error", err)
	}
	if err := m.redis.UpdateState(ctx, roomID, string(StateActive)); err != nil {
		slog.Warn("redis room state update failed", "room", roomID, "error", err)
	}
}

func (m *Manager) onWaiting(roomID string) {
	if m.redis == nil || !m.redis.Enabled() {
		return
	}
	if err := m.redis.UpdateState(context.Background(), roomID, string(StateWaiting)); err != nil {
		slog.Warn("redis room state update failed", "room", roomID, "error", err)
	}
}

func (m *Manager) Close() {
	if m.redis != nil {
		_ = m.redis.Close()
	}
}
