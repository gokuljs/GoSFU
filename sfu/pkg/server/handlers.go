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
	Sdp webrtc.SessionDescription `json:"sdp"`
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
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	var req joinRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	result, err := rm.HandleJoin(req.Sdp)
	if err != nil {
		switch err {
		case room.ErrRoomFull:
			http.Error(w, "room full", http.StatusConflict)
		case room.ErrRoomClosed:
			http.Error(w, "room closed", http.StatusGone)
		default:
			slog.Error("join failed", "room", roomId, "error", err)
			http.Error(w, "join failed", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, joinRoomResponse{
		Sdp:           result.Sdp,
		ParticipantId: result.ParticipantId,
		RoomId:        result.RoomId,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	setCORS(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
