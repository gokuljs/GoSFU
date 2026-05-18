package server

import (
	"net/http"
	"strconv"
)

type Server struct {
	port    int
	sdpChan chan string
}

func HttpSdpServer(port int) chan string {
	sdpChan := make(chan string)
	srv := &Server{
		port:    port,
		sdpChan: sdpChan,
	}
	srv.setupRoutes()
	go func() {
		panic(http.ListenAndServe(":"+strconv.Itoa(port), nil))
	}()
	return sdpChan
}
