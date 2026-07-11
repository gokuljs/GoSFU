package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/pkg/sfu"
	"github.com/gokuljs/goSfu/pkg/transcript"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type sessionConfig struct {
	clientID     int
	baseURL      string
	wavPath      string
	systemPrompt string
	duration     time.Duration
	turns        int
}

type sessionResult struct {
	clientID      int
	roomID        string
	participantID string
	agentTurns    int
	err           error
}

type streamMessage struct {
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

func runSession(ctx context.Context, cfg sessionConfig) sessionResult {
	result := sessionResult{clientID: cfg.clientID}
	api := newAPIClient(cfg.baseURL)

	roomID, err := api.createRoom()
	if err != nil {
		result.err = fmt.Errorf("client %d create room: %w", cfg.clientID, err)
		return result
	}
	result.roomID = roomID
	defer func() {
		if err := api.deleteRoom(roomID); err != nil {
			slog.Warn("delete room failed", "client", cfg.clientID, "room", roomID, "error", err)
		}
	}()

	iceServers, err := api.iceConfig()
	if err != nil {
		result.err = fmt.Errorf("client %d ice-config: %w", cfg.clientID, err)
		return result
	}

	pc, err := sfu.CreatePeerConnectionWithInterceptors(iceServers)
	if err != nil {
		result.err = fmt.Errorf("client %d peer connection: %w", cfg.clientID, err)
		return result
	}
	defer pc.Close()

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio", "mic",
	)
	if err != nil {
		result.err = fmt.Errorf("client %d audio track: %w", cfg.clientID, err)
		return result
	}

	sender, err := pc.AddTrack(track)
	if err != nil {
		result.err = fmt.Errorf("client %d add track: %w", cfg.clientID, err)
		return result
	}
	go drainRTCP(ctx, sender)

	connected := make(chan struct{})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			select {
			case <-connected:
			default:
				close(connected)
			}
		}
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		result.err = fmt.Errorf("client %d create offer: %w", cfg.clientID, err)
		return result
	}
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		result.err = fmt.Errorf("client %d set local description: %w", cfg.clientID, err)
		return result
	}

	gatherCtx, gatherCancel := context.WithTimeout(ctx, 5*time.Second)
	defer gatherCancel()
	select {
	case <-gatherComplete:
	case <-gatherCtx.Done():
		slog.Warn("ice gathering timeout; joining with partial candidates",
			"client", cfg.clientID,
			"room", roomID,
		)
	}

	local := pc.LocalDescription()
	if local == nil {
		result.err = fmt.Errorf("client %d missing local description", cfg.clientID)
		return result
	}

	join, err := api.joinRoom(roomID, *local, cfg.systemPrompt)
	if err != nil {
		result.err = fmt.Errorf("client %d join: %w", cfg.clientID, err)
		return result
	}
	result.participantID = join.ParticipantId

	if err := pc.SetRemoteDescription(join.Sdp); err != nil {
		result.err = fmt.Errorf("client %d set remote description: %w", cfg.clientID, err)
		return result
	}

	source, err := newAudioSource(cfg.wavPath)
	if err != nil {
		result.err = fmt.Errorf("client %d audio source: %w", cfg.clientID, err)
		return result
	}
	encoder, err := audio.NewOpusEncoder(audio.WebrtcSampleRate, audio.ChannelsMono)
	if err != nil {
		result.err = fmt.Errorf("client %d opus encoder: %w", cfg.clientID, err)
		return result
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		pumpAudio(sessionCtx, track, source, encoder)
	}()

	turnsDone := make(chan int, 1)
	go func() {
		defer wg.Done()
		turns := watchTranscripts(sessionCtx, cfg.baseURL, roomID, cfg.turns)
		turnsDone <- turns
	}()

	select {
	case <-connected:
		slog.Info("client connected",
			"client", cfg.clientID,
			"room", roomID,
			"participant", join.ParticipantId,
		)
	case <-time.After(15 * time.Second):
		result.err = fmt.Errorf("client %d timed out waiting for connection", cfg.clientID)
		return result
	case <-ctx.Done():
		result.err = ctx.Err()
		return result
	}

	deadline := time.After(cfg.duration)
	if cfg.turns > 0 {
		select {
		case result.agentTurns = <-turnsDone:
		case <-deadline:
		case <-ctx.Done():
			result.err = ctx.Err()
			return result
		}
	} else {
		select {
		case <-deadline:
		case <-ctx.Done():
			result.err = ctx.Err()
			return result
		}
	}

	cancel()
	wg.Wait()

	if cfg.turns > 0 && result.agentTurns < cfg.turns {
		result.err = fmt.Errorf("client %d saw %d agent turns, wanted %d",
			cfg.clientID, result.agentTurns, cfg.turns)
	}

	slog.Info("client finished",
		"client", cfg.clientID,
		"room", roomID,
		"agent_turns", result.agentTurns,
	)
	return result
}

func pumpAudio(ctx context.Context, track *webrtc.TrackLocalStaticSample, source audioSource, encoder *audio.OpusEncoder) {
	ticker := time.NewTicker(audio.FrameDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			frame, err := source.NextFrame()
			if err != nil {
				slog.Warn("audio frame", "error", err)
				continue
			}
			payload, err := encoder.Encode(frame)
			if err != nil {
				slog.Warn("opus encode", "error", err)
				continue
			}
			if err := track.WriteSample(media.Sample{
				Data:     payload,
				Duration: audio.FrameDuration,
			}); err != nil {
				return
			}
		}
	}
}

func watchTranscripts(ctx context.Context, baseURL, roomID string, targetTurns int) int {
	if targetTurns <= 0 {
		return 0
	}

	wsURL := strings.Replace(baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL = strings.TrimRight(wsURL, "/") + "/room/" + roomID + "/stream"

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		slog.Warn("transcript stream dial failed", "room", roomID, "error", err)
		return 0
	}
	defer conn.Close(websocket.StatusNormalClosure, "load test done")

	agentTurns := 0
	seenTurns := make(map[int]struct{})

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return agentTurns
		}

		var msg streamMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Channel != "transcript" {
			continue
		}

		var update transcript.Update
		if err := json.Unmarshal(msg.Data, &update); err != nil {
			continue
		}
		if update.Speaker != transcript.SpeakerAgent || !update.Final {
			continue
		}
		if _, ok := seenTurns[update.Turn]; ok {
			continue
		}
		seenTurns[update.Turn] = struct{}{}
		agentTurns++
		slog.Info("agent transcript",
			"room", roomID,
			"turn", update.Turn,
			"text", update.Text,
		)
		if agentTurns >= targetTurns {
			return agentTurns
		}
	}
}

func drainRTCP(ctx context.Context, sender *webrtc.RTPSender) {
	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if _, _, err := sender.Read(buf); err != nil {
			return
		}
	}
}

func waitForServer(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		res, err := client.Get(strings.TrimRight(baseURL, "/") + "/ice-config")
		if err == nil && res.StatusCode == http.StatusOK {
			res.Body.Close()
			return nil
		}
		if res != nil {
			res.Body.Close()
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("server not reachable at %s within %s", baseURL, timeout)
}
