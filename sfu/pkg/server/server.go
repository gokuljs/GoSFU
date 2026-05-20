package server

import (
	"fmt"
	"net/http"

	"github.com/gokuljs/goSfu/pkg/room"
)

type Server struct {
	port  int
	rooms *room.Manager
}

func New(port int, rooms *room.Manager) *Server {
	return &Server{port: port, rooms: rooms}
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, s.routes())
}
