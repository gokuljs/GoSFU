package server

import "net/http"

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /room/create", s.handleCreateRoom)
	mux.HandleFunc("OPTIONS /room/create", s.handleCreateRoom)
	mux.HandleFunc("POST /room/{id}/join", s.handleJoinRoom)
	mux.HandleFunc("OPTIONS /room/{id}/join", s.handleJoinRoom)
	mux.HandleFunc("GET /room/{id}/debug", s.handleRoomDebug)
	mux.HandleFunc("OPTIONS /room/{id}/debug", s.handleRoomDebug)
	mux.HandleFunc("GET /room/{id}/transcript", s.handleRoomTranscript)
	mux.HandleFunc("OPTIONS /room/{id}/transcript", s.handleRoomTranscript)
	return mux
}
