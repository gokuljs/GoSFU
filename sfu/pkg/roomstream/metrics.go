package roomstream

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// MetricUpdate is one numeric pipeline measurement for the metrics channel.
type MetricUpdate struct {
	ID    string         `json:"id"`
	TS    time.Time      `json:"ts"`
	Room  string         `json:"room"`
	Turn  int            `json:"turn"`
	Stage string         `json:"stage"` // stt | llm | tts
	Name  string         `json:"name"`
	Value float64        `json:"value"`
	Unit  string         `json:"unit"` // ms | count | tokens
	Meta  map[string]any `json:"meta,omitempty"`
}

// MetricsPublisher emits structured measurements to the room stream.
type MetricsPublisher interface {
	PublishMetric(room string, u MetricUpdate)
}

func (h *Hub) PublishMetric(room string, u MetricUpdate) {
	if h == nil || room == "" {
		return
	}
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	if u.TS.IsZero() {
		u.TS = time.Now().UTC()
	}
	u.Room = room

	slog.Debug("metric created",
		"room", room,
		"turn", u.Turn,
		"stage", u.Stage,
		"name", u.Name,
		"value", u.Value,
		"unit", u.Unit,
	)

	h.publish(ChannelMetrics, room, u, false)
}

// EmitMetric is a convenience helper for orchestrator call sites.
func (h *Hub) EmitMetric(room string, turn int, stage, name, unit string, value float64, meta map[string]any) {
	h.PublishMetric(room, MetricUpdate{
		Turn:  turn,
		Stage: stage,
		Name:  name,
		Value: value,
		Unit:  unit,
		Meta:  meta,
	})
}
