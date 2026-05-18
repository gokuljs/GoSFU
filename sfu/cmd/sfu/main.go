package main

import (
	"flag"

	"github.com/gokuljs/goSfu/pkg/server"
)

func main() {
	port := flag.Int("port", 8080, "http server port")
	flag.Parse()
	sdpChan := server.HttpSdpServer(*port)
}
