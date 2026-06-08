// Package transcript fans out live conversation text to browser clients over
// a dedicated WebSocket. Unlike sessiondebug, updates are never dropped.
package transcript

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const historyLimit = 50

const (
	SpeakerUser  = "user"
	SpeakerAgent = "agent"
)

// Update is one transcript upsert for a speaker/turn pair.
type Update struct {
	Speaker string    `json:"speaker"`
	Text    string    `json:"text"`
	Final   bool      `json:"final"`
	Turn    int       `json:"turn"`
	TS      time.Time `json:"ts"`
}

// Publisher emits transcript updates for a room.
type Publisher interface {
	Publish(room string, u Update)
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Update]struct{}
	history     map[string][]Update
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[chan Update]struct{}),
		history:     make(map[string][]Update),
	}
}

func (h *Hub) Publish(room string, u Update) {
	if h == nil || strings.TrimSpace(room) == "" {
		return
	}
	u.Text = strings.TrimSpace(u.Text)
	if u.Text == "" {
		return
	}
	if u.TS.IsZero() {
		u.TS = time.Now().UTC()
	}

	h.mu.Lock()
	history := append(h.history[room], u)
	if len(history) > historyLimit {
		history = history[len(history)-historyLimit:]
	}
	h.history[room] = history

	for ch := range h.subscribers[room] {
		ch <- u
	}
	h.mu.Unlock()
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, room string) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "transcript stream closed")

	ctx := r.Context()
	updates, unsubscribe := h.Subscribe(room)
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case u, ok := <-updates:
			if !ok {
				return
			}
			payload, err := json.Marshal(u)
			if err != nil {
				continue
			}
			if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
				return
			}
		}
	}
}

func (h *Hub) Subscribe(room string) (<-chan Update, func()) {
	ch := make(chan Update, 64)

	h.mu.Lock()
	if h.subscribers[room] == nil {
		h.subscribers[room] = make(map[chan Update]struct{})
	}
	h.subscribers[room][ch] = struct{}{}
	history := append([]Update(nil), h.history[room]...)
	h.mu.Unlock()

	for _, u := range history {
		ch <- u
	}

	return ch, func() {
		h.mu.Lock()
		if subs := h.subscribers[room]; subs != nil {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(h.subscribers, room)
			}
		}
		close(ch)
		h.mu.Unlock()
	}
}
