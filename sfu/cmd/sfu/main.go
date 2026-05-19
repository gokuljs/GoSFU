package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/gokuljs/goSfu/pkg/config"
	"github.com/gokuljs/goSfu/pkg/logger"
	"github.com/gokuljs/goSfu/pkg/server"
	"github.com/gokuljs/goSfu/pkg/sfu"
	"github.com/pion/webrtc/v4"
)

func main() {
	port := flag.Int("port", config.DEFAULT_PORT, "http server port")
	env := flag.String("env", "development", "environment: development or production")
	flag.Parse()

	log := logger.Init(logger.EnvFromString(*env))
	log.Info("starting Go SFU server", "port", *port, "env", *env)

	sdpChan := server.HttpSdpServer(*port)
	log.Info("signaling server ready", "addr", "http://localhost:"+itoa(*port))

	offer := webrtc.SessionDescription{}
	log.Info("waiting for publisher SDP offer...")
	server.Decode(<-sdpChan, &offer)
	log.Debug("received publisher offer", "type", offer.Type.String())

	peerConnection, err := sfu.CreatePeerConnectionWithInterceptors(config.STUN_SERVER)
	if err != nil {
		log.Error("failed to create peer connection", "error", err)
		os.Exit(1)
	}

	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			log.Warn("failed to close peer connection", "error", cErr)
		}
	}()

	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		log.Error("failed to add video transceiver", "error", err)
		os.Exit(1)
	}

	localTrackChan := make(chan *webrtc.TrackLocalStaticRTP)

	peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) { //nolint: revive
		log.Info("received remote track",
			"codec", remoteTrack.Codec().MimeType,
			"ssrc", remoteTrack.SSRC(),
		)

		localTrack, newTrackErr := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "video", "pion")
		if newTrackErr != nil {
			log.Error("failed to create local track", "error", newTrackErr)
			return
		}
		localTrackChan <- localTrack

		rtpBuf := make([]byte, 1400)
		for {
			i, _, readErr := remoteTrack.Read(rtpBuf)
			if readErr != nil {
				log.Error("remote track read error", "error", readErr)
				return
			}

			if _, err = localTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				log.Error("local track write error", "error", err)
				return
			}
		}
	})

	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		log.Error("failed to set remote description", "error", err)
		os.Exit(1)
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Error("failed to create answer", "error", err)
		os.Exit(1)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		log.Error("failed to set local description", "error", err)
		os.Exit(1)
	}

	<-gatherComplete
	log.Info("ICE gathering complete for publisher")

	slog.Debug("publisher answer SDP", "sdp", server.Encode(peerConnection.LocalDescription()))

	localTrack := <-localTrackChan
	log.Info("local track ready, accepting subscribers")

	for {
		log.Info("waiting for subscriber SDP offer...")

		recvOnlyOffer := webrtc.SessionDescription{}
		server.Decode(<-sdpChan, &recvOnlyOffer)
		log.Debug("received subscriber offer", "type", recvOnlyOffer.Type.String())

		peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{
					URLs: []string{config.STUN_SERVER},
				},
			},
		})
		if err != nil {
			log.Error("failed to create subscriber peer connection", "error", err)
			continue
		}

		rtpSender, err := peerConnection.AddTrack(localTrack)
		if err != nil {
			log.Error("failed to add track to subscriber", "error", err)
			continue
		}

		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

		err = peerConnection.SetRemoteDescription(recvOnlyOffer)
		if err != nil {
			log.Error("subscriber: failed to set remote description", "error", err)
			continue
		}

		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			log.Error("subscriber: failed to create answer", "error", err)
			continue
		}

		gatherComplete = webrtc.GatheringCompletePromise(peerConnection)

		err = peerConnection.SetLocalDescription(answer)
		if err != nil {
			log.Error("subscriber: failed to set local description", "error", err)
			continue
		}

		<-gatherComplete
		log.Info("subscriber connected, ICE gathering complete")
		slog.Debug("subscriber answer SDP", "sdp", server.Encode(peerConnection.LocalDescription()))
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
