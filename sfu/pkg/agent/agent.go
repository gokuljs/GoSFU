// Package agent owns the conversation layer. It is deliberately transport-
// agnostic: it talks to a transport.Transport (WebRTC today, SIP/Twilio later)
// purely in terms of audio.Frame, and to the STT/LLM/TTS/VAD plugins purely
// through their interfaces. No vendor SDK and no pion/webrtc imports live here.
package agent

import (
	"context"
	"log/slog"

	"github.com/gokuljs/goSfu/pkg/agent/transport"
)

// Agent is the lifecycle owner: it wires a transport to an orchestrator and
// starts/stops both as one unit.
type Agent struct {
	ctx       context.Context
	cancel    context.CancelFunc
	transport transport.Transport
	orch      *Orchestrator
}

// New builds an agent over any transport with a fully-resolved plugin config
// (see NewConfig). The caller owns the transport's underlying
// endpoint (e.g. the PeerConnection) and remote-track delivery.
func New(ctx context.Context, t transport.Transport, cfg Config) *Agent {
	ctx, cancel := context.WithCancel(ctx)
	return &Agent{
		ctx:       ctx,
		cancel:    cancel,
		transport: t,
		orch:      NewOrchestrator(cfg, t),
	}
}

// Start brings up media flow then launches the conversation loop. Call once
// the endpoint is connected.
func (a *Agent) Start() {
	slog.Info("agent starting")
	if err := a.transport.Start(a.ctx); err != nil {
		slog.Error("transport start failed", "error", err)
		return
	}
	go a.orch.Run(a.ctx)
}

// Stop cancels every goroutine in the agent subtree and releases the transport.
func (a *Agent) Stop() {
	a.cancel()
	_ = a.transport.Close()
}
