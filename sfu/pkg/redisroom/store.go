package redisroom

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RoomKeyPrefix     = "room:"
	LiveChannelPrefix = "roomstream:live:"
	RoomTTL           = time.Hour
)

type Record struct {
	Owner     string    `json:"owner"`
	CreatedAt time.Time `json:"created_at"`
	State     string    `json:"state"`
}

type Store struct {
	client *redis.Client
	nodeID string
}

func NewStore(url, nodeID string) (*Store, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, nil
	}

	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	if strings.TrimSpace(nodeID) == "" {
		nodeID = "local"
	}

	return &Store{client: client, nodeID: nodeID}, nil
}

func (s *Store) Client() *redis.Client {
	if s == nil {
		return nil
	}
	return s.client
}

func (s *Store) NodeID() string {
	if s == nil {
		return ""
	}
	return s.nodeID
}

func (s *Store) Enabled() bool {
	return s != nil && s.client != nil
}

func (s *Store) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *Store) roomKey(roomID string) string {
	return RoomKeyPrefix + roomID
}

func LiveChannel(roomID string) string {
	return LiveChannelPrefix + roomID
}

func (s *Store) Register(ctx context.Context, roomID, state string) error {
	if !s.Enabled() || strings.TrimSpace(roomID) == "" {
		return nil
	}
	rec := Record{
		Owner:     s.nodeID,
		CreatedAt: time.Now().UTC(),
		State:     state,
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.roomKey(roomID), raw, RoomTTL).Err()
}

func (s *Store) UpdateState(ctx context.Context, roomID, state string) error {
	if !s.Enabled() || strings.TrimSpace(roomID) == "" {
		return nil
	}
	key := s.roomKey(roomID)
	raw, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return s.Register(ctx, roomID, state)
		}
		return err
	}
	var rec Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return err
	}
	rec.State = state
	next, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, next, RoomTTL).Err()
}

func (s *Store) Refresh(ctx context.Context, roomID string) error {
	if !s.Enabled() || strings.TrimSpace(roomID) == "" {
		return nil
	}
	return s.client.Expire(ctx, s.roomKey(roomID), RoomTTL).Err()
}

func (s *Store) Exists(ctx context.Context, roomID string) (bool, error) {
	if !s.Enabled() || strings.TrimSpace(roomID) == "" {
		return false, nil
	}
	n, err := s.client.Exists(ctx, s.roomKey(roomID)).Result()
	return n > 0, err
}

func (s *Store) Delete(ctx context.Context, roomID string) error {
	if !s.Enabled() || strings.TrimSpace(roomID) == "" {
		return nil
	}
	return s.client.Del(ctx, s.roomKey(roomID)).Err()
}
