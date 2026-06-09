package roomstream

import (
	"time"

	"github.com/gokuljs/goSfu/pkg/roomquota"
)

type QuotaUpdate struct {
	Room       string    `json:"room"`
	Limit      int       `json:"limit"`
	Used       int       `json:"used"`
	ActiveTurn int       `json:"activeTurn,omitempty"`
	Exhausted  bool      `json:"exhausted"`
	Message    string    `json:"message"`
	TS         time.Time `json:"ts"`
}

type QuotaPublisher interface {
	PublishQuota(room string, state roomquota.State)
}

func QuotaUpdateFromState(state roomquota.State) QuotaUpdate {
	return QuotaUpdate{
		Room:       state.Room,
		Limit:      state.Limit,
		Used:       state.Used,
		ActiveTurn: state.ActiveTurn,
		Exhausted:  state.Exhausted,
		Message:    state.Message,
		TS:         state.TS,
	}
}
