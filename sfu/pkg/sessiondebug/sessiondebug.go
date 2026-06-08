package sessiondebug

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

const historyLimit = 300

type Event struct {
	ID       string         `json:"id"`
	TS       time.Time      `json:"ts"`
	Room     string         `json:"room"`
	Type     string         `json:"type"`
	Category string         `json:"category"`
	Level    string         `json:"level"`
	Message  string         `json:"message"`
	Source   string         `json:"source"`
	Turn     *int           `json:"turn,omitempty"`
	Attrs    map[string]any `json:"attrs,omitempty"`
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Event]struct{}
	history     map[string][]Event
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[chan Event]struct{}),
		history:     make(map[string][]Event),
	}
}

func (h *Hub) Publish(event Event) {
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
		event.Category = CategoryFor(event.Type)
	}
	if event.Level == "" {
		event.Level = "info"
	}
	if event.Source == "" {
		event.Source = "server"
	}

	h.mu.Lock()
	history := append(h.history[event.Room], event)
	if len(history) > historyLimit {
		history = history[len(history)-historyLimit:]
	}
	h.history[event.Room] = history

	for ch := range h.subscribers[event.Room] {
		select {
		case ch <- event:
		default:
			// Keep media/pipeline paths non-blocking if a debug client falls behind.
		}
	}
	h.mu.Unlock()
}

func (h *Hub) PublishEvent(room, typ, level, msg string, attrs map[string]any) {
	h.Publish(Event{
		Room:    room,
		Type:    typ,
		Level:   level,
		Message: msg,
		Source:  "server",
		Attrs:   attrs,
		Turn:    TurnFromAttrs(attrs),
	})
}

func (h *Hub) PublishPipeline(level slog.Level, event, msg string, attrs ...any) {
	values := AttrsMap(attrs...)
	room, _ := values["room"].(string)
	h.Publish(Event{
		Room:     room,
		Type:     event,
		Category: CategoryFor(event),
		Level:    LevelString(level),
		Message:  msg,
		Source:   "pipeline",
		Turn:     TurnFromAttrs(values),
		Attrs:    values,
	})
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, room string) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "debug stream closed")

	ctx := r.Context()
	events, unsubscribe := h.Subscribe(room)
	defer unsubscribe()

	h.PublishEvent(room, "debug.websocket.connected", "info", "Debug WebSocket connected", nil)
	defer h.PublishEvent(room, "debug.websocket.disconnected", "info", "Debug WebSocket disconnected", nil)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
				return
			}
		}
	}
}

func (h *Hub) Subscribe(room string) (<-chan Event, func()) {
	ch := make(chan Event, 128)

	h.mu.Lock()
	if h.subscribers[room] == nil {
		h.subscribers[room] = make(map[chan Event]struct{})
	}
	h.subscribers[room][ch] = struct{}{}
	history := append([]Event(nil), h.history[room]...)
	h.mu.Unlock()

	if len(history) > cap(ch) {
		history = history[len(history)-cap(ch):]
	}
	for _, event := range history {
		ch <- event
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

func AttrsMap(attrs ...any) map[string]any {
	values := make(map[string]any, len(attrs)/2)
	for i := 0; i+1 < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		if !ok || key == "" {
			continue
		}
		values[key] = normalize(attrs[i+1])
	}
	return values
}

func normalize(v any) any {
	switch value := v.(type) {
	case error:
		return value.Error()
	case fmt.Stringer:
		return value.String()
	default:
		return value
	}
}

func TurnFromAttrs(attrs map[string]any) *int {
	value, ok := attrs["turn"]
	if !ok {
		return nil
	}
	var turn int
	switch v := value.(type) {
	case int:
		turn = v
	case int64:
		turn = int(v)
	case uint64:
		turn = int(v)
	case float64:
		turn = int(v)
	default:
		return nil
	}
	return &turn
}

func CategoryFor(event string) string {
	if strings.HasPrefix(event, "pipeline.") {
		parts := strings.Split(event, ".")
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	parts := strings.Split(event, ".")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "session"
}

func LevelString(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "error"
	case level >= slog.LevelWarn:
		return "warn"
	case level <= slog.LevelDebug:
		return "debug"
	default:
		return "info"
	}
}
