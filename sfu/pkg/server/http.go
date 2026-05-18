package server

import (
	"fmt"
	"io"
	"net/http"
)

func (s *Server) setupRoutes() {
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		fmt.Fprintf(res, "done")
		s.sdpChan <- string(body)
	})
}
