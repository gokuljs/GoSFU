# GoSFU

A forkable WebRTC voice agent server in Go.

Handles the audio transport, turn orchestration, pacing, and barge-in so you can focus on your agent logic.

[![license](https://img.shields.io/badge/license-MIT-2ea44f)](LICENSE) [![version](https://img.shields.io/badge/version-v0-orange)](https://github.com/gokuljs/goSfu) [![demo](https://img.shields.io/badge/demo-YouTube-FF0000?logo=youtube&logoColor=white)](https://youtu.be/ZRbSO5N5Q8o?si=rxhzvA9pHx142EFH)

The interesting part is not that it connects a speech model, a language model, and a voice model. That wiring is the easy part. The hard part is what happens between the microphone and the speaker while the clock is running.

Browser audio arrives as Opus over WebRTC. The server decodes it into PCM, resamples it for the parts of the system that need a different clock rate, watches for speech boundaries, waits for a complete user turn, streams a response, turns that response back into audio, reframes it into 20 ms chunks, paces it, and sends it back over WebRTC without stuttering.

If the user interrupts while the agent is speaking, the current response is cancelled and the queued audio is cleared. The system goes back to listening.

That loop is the project.

Most voice agent examples hide the part where time becomes a problem. A text system can wait. A voice system cannot. If audio arrives too early, too late, in the wrong sample rate, in uneven chunks, or after the user has already started talking again, the user hears the bug immediately. This repo keeps those moving parts visible.

It is meant to be forked. Read the orchestrator, swap the providers, change the turn logic, deploy your own version. The debug console shows what the system is doing while the call is running, so you can see the effect of your changes immediately.

I wrote more about the audio side in [When Latency Becomes Audible](https://gokuljs.com/blogs/when-latency-becomes-audible).

**Features:**

- **WebRTC transport** — Opus encode/decode, 48 kHz to model-friendly resampling, 20 ms frame buffering
- **Turn orchestration** — single-owner state machine for listening, responding, and barge-in
- **Audio pacing** — frame pacer that turns bursty generated audio into a steady playout clock with backpressure
- **Streaming pipeline** — chunked response output, streaming resampling, fade-in/out at utterance boundaries
- **Provider interfaces** — pluggable LLM, STT, TTS, and VAD behind clean contracts. Swap vendors without touching the transport layer
- **Live debug console** — browser UI for transcripts, events, latency metrics, connection state, and session quota
- **Redis or in-memory** — Redis-backed room state and event fanout for multi-process deployments, with an in-memory fallback for local dev

## The Core Loop

```text
Browser mic
  -> WebRTC Opus packets
  -> server-side PCM frames
  -> turn detection and transcript collection
  -> response generation
  -> generated PCM audio
  -> resample, reframe, fade, pace
  -> WebRTC Opus packets
  -> browser speaker
```

While the agent is speaking, inbound audio is still monitored. If speech starts again, the server cancels the response path, clears the playout buffer, and returns to listening.

## Repository Layout

```text
sfu/
  cmd/sfu/              server entrypoint
  pkg/agent/            conversation orchestrator
  pkg/agent/audio/      frames, resampling, buffering, pacing, Opus helpers
  pkg/agent/transport/  WebRTC transport boundary
  pkg/room/             room lifecycle
  pkg/roomstream/       live transcript/debug/metric streams
  pkg/redisroom/        Redis room registry and pub/sub channel names
  plugins/              provider interfaces and implementations

sfu-frontend/
  src/pages/room.tsx    debug console
  src/hooks/            WebRTC and room stream hooks
  src/components/       metrics, transcript, session, and audio UI
```

## Quickstart

**Requirements:** Go 1.26+, Node.js 20+, and API keys for the demo providers (OpenAI, Deepgram, Rime). ONNX Runtime and a Silero model are needed for voice activity detection. Redis is optional — leave `REDIS_URL` unset to use the in-memory fallback.

Stub providers exist in the repo, but the demo path is wired to OpenAI, Deepgram, Rime, and Silero in `sfu/pkg/room/room.go`.

```bash
# frontend
cd sfu-frontend
npm install

# backend
cd ../sfu
go mod download
cp .env.example .env
```

Edit `sfu/.env` with your API keys (`OPENAI_API_KEY`, `DEEPGRAM_API_KEY`, `RIME_API_KEY`, `SILERO_MODEL_PATH`). See `.env.example` for the full list.

```bash
# start frontend
cd sfu-frontend
npm run dev

# start backend (in another terminal)
cd sfu
go run ./cmd/sfu
```

Open `http://localhost:3000`, click connect, and allow microphone access.

## What To Look At

The useful parts of the repo are the places where timing decisions are made.

- `sfu/pkg/agent/orchestrator.go` — owns the listening/responding state and handles barge-in
- `sfu/pkg/agent/audio/pacer.go` — converts bursty generated audio into a steady 20 ms playout clock
- `sfu/pkg/agent/audio/resample.go` — frame-level and streaming resampling
- `sfu/pkg/agent/transport/webrtc.go` — separates the WebRTC boundary from the agent logic
- `sfu/pkg/roomstream/hub.go` — streams transcripts, debug events, metrics, and quota updates
- `sfu-frontend/src/pages/room.tsx` — the live debug console
- `sfu-frontend/src/components/metrics-panel.tsx` — latency and pipeline metrics

## Current State

This is early software. It works end to end, but the goal is to keep the core readable and honest rather than to cover every production edge case.

## Contributing

The direction of this project should come from what people actually try to build with it.

If you want a feature or hit a limitation, open an issue with the use case. Provider implementations, transport options, deployment guides, and orchestration changes are all welcome start with an issue so the approach is clear.

Small fixes and documentation improvements can go straight to a PR.
