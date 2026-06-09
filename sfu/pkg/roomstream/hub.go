package roomstream

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/gokuljs/goSfu/pkg/roomquota"
	"github.com/gokuljs/goSfu/pkg/sessiondebug"
	"github.com/gokuljs/goSfu/pkg/transcript"
	"github.com/google/uuid"
)

const (
	ChannelTranscript = "transcript"
	ChannelDebug      = "debug"
	ChannelMetrics    = "metrics"
	ChannelQuota      = "quota"

	transcriptHistoryLimit = 50
	debugHistoryLimit      = 300
	metricsHistoryLimit    = 200
	quotaHistoryLimit      = 20
)

// Message is the envelope sent over the room WebSocket.
type Message struct {
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

type queuedMessage struct {
	channel string
	raw     json.RawMessage
}

// Hub multiplexes transcript, debug, and metrics onto one WebSocket per room.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan queuedMessage]struct{}
	history     map[string][]queuedMessage
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[chan queuedMessage]struct{}),
		history:     make(map[string][]queuedMessage),
	}
}

func (h *Hub) PublishTranscript(room string, u transcript.Update) {
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
	h.publish(ChannelTranscript, room, u, false)
}

// Publish implements transcript.Publisher.
func (h *Hub) Publish(room string, u transcript.Update) {
	h.PublishTranscript(room, u)
}

func (h *Hub) PublishDebug(event sessiondebug.Event) {
	if h == nil || strings.TrimSpace(event.Room) == "" {
		return
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.TS.IsZero() {
		event.TS = time.Now().UTC()
	}
	if event.Category == "" {
		event.Category = sessiondebug.CategoryFor(event.Type)
	}
	if event.Level == "" {
		event.Level = "info"
	}
	if event.Source == "" {
		event.Source = "server"
	}
	h.publish(ChannelDebug, event.Room, event, true)
}

func (h *Hub) PublishEvent(room, typ, level, msg string, attrs map[string]any) {
	h.PublishDebug(sessiondebug.Event{
		Room:    room,
		Type:    typ,
		Level:   level,
		Message: msg,
		Source:  "server",
		Attrs:   attrs,
		Turn:    sessiondebug.TurnFromAttrs(attrs),
	})
}

func (h *Hub) PublishQuota(room string, state roomquota.State) {
	if h == nil || strings.TrimSpace(room) == "" {
		return
	}
	h.publish(ChannelQuota, room, QuotaUpdateFromState(state), false)
}

func (h *Hub) PublishPipeline(level slog.Level, event, msg string, attrs ...any) {
	values := sessiondebug.AttrsMap(attrs...)
	room, _ := values["room"].(string)
	h.PublishDebug(sessiondebug.Event{
		Room:     room,
		Type:     event,
		Category: sessiondebug.CategoryFor(event),
		Level:    sessiondebug.LevelString(level),
		Message:  msg,
		Source:   "pipeline",
		Turn:     sessiondebug.TurnFromAttrs(values),
		Attrs:    values,
	})
}

func (h *Hub) publish(channel, room string, data any, droppable bool) {
	raw, err := json.Marshal(data)
	if err != nil {
		slog.Warn("roomstream marshal failed", "channel", channel, "room", room, "error", err)
		return
	}

	msg := queuedMessage{channel: channel, raw: raw}

	h.mu.Lock()
	history := append(h.history[room], msg)
	limit := historyLimit(channel)
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	h.history[room] = history

	for ch := range h.subscribers[room] {
		if droppable {
			select {
			case ch <- msg:
			default:
			}
		} else {
			ch <- msg
		}
	}
	h.mu.Unlock()

	slog.Debug("stream sent",
		"channel", channel,
		"room", room,
		"bytes", len(raw),
	)
}

func historyLimit(channel string) int {
	switch channel {
	case ChannelTranscript:
		return transcriptHistoryLimit
	case ChannelMetrics:
		return metricsHistoryLimit
	case ChannelQuota:
		return quotaHistoryLimit
	default:
		return debugHistoryLimit
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, room string) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "room stream closed")

	ctx := r.Context()
	msgs, unsubscribe := h.Subscribe(room)
	defer unsubscribe()

	h.PublishEvent(room, "stream.websocket.connected", "info", "Room stream WebSocket connected", nil)
	defer h.PublishEvent(room, "stream.websocket.disconnected", "info", "Room stream WebSocket disconnected", nil)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			envelope := Message{Channel: msg.channel, Data: msg.raw}
			payload, err := json.Marshal(envelope)
			if err != nil {
				continue
			}
			if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
				return
			}
		}
	}
}

func (h *Hub) Subscribe(room string) (<-chan queuedMessage, func()) {
	ch := make(chan queuedMessage, 256)

	h.mu.Lock()
	if h.subscribers[room] == nil {
		h.subscribers[room] = make(map[chan queuedMessage]struct{})
	}
	h.subscribers[room][ch] = struct{}{}
	history := append([]queuedMessage(nil), h.history[room]...)
	h.mu.Unlock()

	if len(history) > cap(ch) {
		history = history[len(history)-cap(ch):]
	}
	for _, msg := range history {
		ch <- msg
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
