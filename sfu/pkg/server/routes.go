package server

import "net/http"

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /room/create", s.handleCreateRoom)
	mux.HandleFunc("OPTIONS /room/create", s.handleCreateRoom)
	mux.HandleFunc("POST /room/{id}/join", s.handleJoinRoom)
	mux.HandleFunc("OPTIONS /room/{id}/join", s.handleJoinRoom)
	mux.HandleFunc("POST /room/{id}/session/stop", s.handleStopSession)
	mux.HandleFunc("OPTIONS /room/{id}/session/stop", s.handleStopSession)
	mux.HandleFunc("DELETE /room/{id}", s.handleDeleteRoom)
	mux.HandleFunc("OPTIONS /room/{id}", s.handleDeleteRoom)
	mux.HandleFunc("GET /room/{id}/stream", s.handleRoomStream)
	mux.HandleFunc("OPTIONS /room/{id}/stream", s.handleRoomStream)
	return loggingMiddleware(mux)
}
