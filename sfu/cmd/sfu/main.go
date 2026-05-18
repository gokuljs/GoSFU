package main

import (
	"flag"

	"github.com/gokuljs/goSfu/pkg/server"
	"github.com/pion/webrtc/v4"
)

func main() {
	port := flag.Int("port", 8080, "http server port")
	flag.Parse()
	sdpChan := server.HttpSdpServer(*port)
	offer := webrtc.SessionDescription{}
	server.Decode(<-sdpChan, &offer)
}
