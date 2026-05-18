package server

import (
	"encoding/base64"
	"encoding/json"

	"github.com/pion/webrtc/v4"
)

func Decode(in string, obj *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(b, obj); err != nil {
		panic(err)
	}
}
