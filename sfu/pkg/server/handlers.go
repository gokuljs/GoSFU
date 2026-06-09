package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gokuljs/goSfu/pkg/room"
	"github.com/pion/webrtc/v4"
)

type createRoomResponse struct {
	RoomID string `json:"roomId"`
}

type joinRoomRequest struct {
	Sdp          webrtc.SessionDescription `json:"sdp"`
	SystemPrompt string                    `json:"systemPrompt,omitempty"`
}

type joinRoomResponse struct {
	Sdp           webrtc.SessionDescription `json:"sdp"`
	ParticipantId string                    `json:"participantId"`
	RoomId        string                    `json:"roomId"`
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	id := s.rooms.Create()
	slog.Info("room create requested", "room", id)
	writeJSON(w, http.StatusOK, createRoomResponse{RoomID: id})
}

func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	roomId := r.PathValue("id")
	rm, ok := s.rooms.Get(roomId)
	if !ok {
		slog.Warn("join rejected", "room", roomId, "reason", "not_found")
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	var req joinRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	result, err := rm.HandleJoin(req.Sdp, req.SystemPrompt)
	if err != nil {
		switch err {
		case room.ErrRoomFull:
			slog.Warn("join rejected", "room", roomId, "reason", "room_full")
			http.Error(w, "room full", http.StatusConflict)
		case room.ErrRoomClosed:
			slog.Warn("join rejected", "room", roomId, "reason", "room_closed")
			http.Error(w, "room closed", http.StatusGone)
		case room.ErrQuotaExhausted:
			slog.Warn("join rejected", "room", roomId, "reason", "quota_exhausted")
			http.Error(w, "session quota exhausted", http.StatusTooManyRequests)
		default:
			slog.Error("join failed", "room", roomId, "error", err)
			http.Error(w, "join failed", http.StatusInternalServerError)
		}
		return
	}

	slog.Info("participant joined",
		"room", roomId,
		"participant", result.ParticipantId,
	)

	writeJSON(w, http.StatusOK, joinRoomResponse{
		Sdp:           result.Sdp,
		ParticipantId: result.ParticipantId,
		RoomId:        result.RoomId,
	})
}

func (s *Server) handleStopSession(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	roomId := r.PathValue("id")
	rm, ok := s.rooms.Get(roomId)
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	rm.StopSession()
	setCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	roomId := r.PathValue("id")
	rm, ok := s.rooms.Get(roomId)
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	rm.Close()
	setCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRoomStream(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	roomId := r.PathValue("id")
	if _, ok := s.rooms.Get(roomId); !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	s.rooms.Stream().ServeWS(w, r, roomId)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	setCORS(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
