package server

import "net/http"

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /room/create", s.handleCreateRoom)
	mux.HandleFunc("OPTIONS /room/create", s.handleCreateRoom)
	return mux
}
