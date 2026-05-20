package server

import (
	"encoding/json"
	"net/http"

	"github.com/gokuljs/goSfu/pkg/room"
)

type createRoomResponse struct {
	RoomID string `json:"roomId"`
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
	if err := rm.ReserveUser(); err != nil {
		switch err {
		case room.ErrRoomFull:
			http.Error(w, "room full", http.StatusConflict)
		case room.ErrRoomClosed:
			http.Error(w, "room closed", http.StatusGone)
		default:
			http.Error(w, "join failed", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"roomId": roomId,
		"status": "joined",
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
