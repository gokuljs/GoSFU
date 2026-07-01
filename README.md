# GoSFU

Open-source Go infrastructure for the real-time voice AI loop.

A WebRTC voice agent stack focused on the hard parts: audio transport, turn orchestration, pacing, barge-in, and observability.

> Status: Early / Work in Progress
>
> Working end-to-end demo. Not production hardened.

The interesting part is not that it connects a speech model, a language model, and a voice model. That wiring is the easy part. The hard part is what happens between the microphone and the speaker while the clock is running.

Browser audio arrives as Opus over WebRTC. The server decodes it into PCM, resamples it for the parts of the system that need a different clock rate, watches for speech boundaries, waits for a complete user turn, streams a response, turns that response back into audio, reframes it into 20 ms chunks, paces it, and sends it back over WebRTC without stuttering.

If the user interrupts while the agent is speaking, the current response is cancelled and the queued audio is cleared. The system goes back to listening.

That loop is the project.

[![Watch the GoSFU demo on YouTube](https://img.shields.io/badge/Watch%20demo-YouTube-red?style=for-the-badge&logo=youtube&logoColor=white)](https://youtu.be/ZRbSO5N5Q8o?si=rxhzvA9pHx142EFH)

## Why This Exists

Most voice agent examples hide the part where time becomes a problem.

A text system can wait. A voice system cannot. If audio arrives too early, too late, in the wrong sample rate, in uneven chunks, or after the user has already started talking again, the user hears the bug immediately.

This repo keeps those moving parts visible.

It is meant to be read, run, forked, and changed. The code shows the full path from browser microphone input to server-side media processing to paced audio output. The debug console shows what the system is doing while the call is running.

I wrote more about the audio side in [When Latency Becomes Audible](https://gokuljs.com/blogs/when-latency-becomes-audible).

## What It Does

GoSFU runs one browser-to-agent voice session at a time per room.

The browser sends microphone audio to the Go server over WebRTC. The server owns the conversation state. It listens for speech, accumulates the user's turn, starts the response path, and sends the agent's audio back to the browser.

The current system includes:

- WebRTC signaling and audio transport using Pion.
- Opus decode on the inbound path and Opus encode on the outbound path.
- PCM audio as the internal format.
- 48 kHz WebRTC audio converted to model-friendly sample rates where needed.
- A single-owner orchestrator for turn state.
- Barge-in handling when the user talks over the agent.
- Streaming response output split into speakable chunks.
- Streaming resampling for generated audio.
- 20 ms frame buffering for WebRTC.
- Fade-in and fade-out at utterance boundaries to avoid clicks.
- A frame pacer that turns bursty generated audio into a steady playout clock.
- Backpressure so generated audio cannot build up without bound.
- A live browser console for transcripts, events, metrics, connection state, and quota.
- Redis-backed room state and live room event fanout, with an in-memory fallback for local use.

The model providers are intentionally behind interfaces. The important boundary is the audio and orchestration layer, not a specific vendor.

## The Core Loop

The system is not a straight pipeline. It is a small state machine.

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

That detail matters. Without it, the agent feels like a recording that cannot be interrupted.

## Redis

Redis is used for two things.

First, it stores lightweight room state. Each room is registered with an owner node, a creation time, a state, and a TTL. This lets the server know whether a room exists even when the room is not local to the current process.

Second, it carries live room events. Transcripts, debug events, metrics, and quota updates are published to a Redis channel for the room. The browser debug console subscribes to the room stream through the server and receives those updates in real time.

For local development, the server can run without Redis. If `REDIS_URL` is not set, it falls back to in-memory room state and local WebSocket fanout. That is enough for a single local process.

For anything with more than one server process, Redis should be enabled. Otherwise one process will not know about rooms and live events owned by another process.

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

## Requirements

- Go 1.26+
- Node.js 20+
- Redis 7+ if you want shared room state and live event fanout
- API keys for the providers you enable
- ONNX Runtime and a Silero model if you use the Silero VAD provider

The default local provider setup can use stubs for most components. Real calls require the corresponding provider keys.

## Setup

Clone the repo and install the browser dependencies:

```bash
cd sfu-frontend
npm install
```

Install the Go dependencies:

```bash
cd ../sfu
go mod download
```

Create a local environment file:

```bash
cp .env.example .env
```

Edit `sfu/.env` for your local setup.

Important environment variables:

- `PORT`: HTTP server port. Defaults to `8080`.
- `ENV`: logging environment. Usually `local`.
- `REDIS_URL`: Redis connection string, for example `redis://localhost:6379`.
- `NODE_ID`: identifier for this server process. Defaults to `local`.
- `SESSION_MAX_DURATION`: maximum session length, for example `30m`.
- `LLM_PROVIDER`: `stub` or `openai`.
- `STT_PROVIDER`: `stub` or `deepgram`.
- `TTS_PROVIDER`: `stub` or `rime`.
- `VAD_PROVIDER`: `stub` or `silero`.
- `OPENAI_API_KEY`: required when using the OpenAI provider.
- `DEEPGRAM_API_KEY`: required when using the Deepgram provider.
- `RIME_API_KEY`: required when using the Rime provider.
- `ONNXRUNTIME_LIB_PATH`: path to the ONNX Runtime library when using Silero.
- `SILERO_MODEL_PATH`: path to the Silero model file when using Silero.

See `sfu/.env.example` for provider-specific defaults.

## Running Redis

On macOS:

```bash
brew install redis
brew services start redis
redis-cli ping
```

The last command should print:

```text
PONG
```

If you prefer not to run Redis locally, leave `REDIS_URL` unset. The server will use the in-memory fallback. Use Redis when you want the same behavior as a multi-process deployment.

## Running The App

Start Redis first if you are using it.

Start the browser client:

```bash
cd sfu-frontend
npm run dev
```

Start the Go server:

```bash
cd sfu
go run ./cmd/sfu
```

Open:

```text
http://localhost:3000
```

Click connect and allow microphone access.

## What To Look At

The useful parts of the repo are the places where timing decisions are made.

- `sfu/pkg/agent/orchestrator.go`: owns the listening/responding state and handles barge-in.
- `sfu/pkg/agent/audio/pacer.go`: converts bursty generated audio into a steady 20 ms playout clock.
- `sfu/pkg/agent/audio/resample.go`: contains both frame-level and streaming resampling.
- `sfu/pkg/agent/transport/webrtc.go`: separates the WebRTC boundary from the agent logic.
- `sfu/pkg/roomstream/hub.go`: streams transcripts, debug events, metrics, and quota updates.
- `sfu/pkg/redisroom/store.go`: keeps room state in Redis and defines live room channels.
- `sfu-frontend/src/pages/room.tsx`: renders the live debug console.
- `sfu-frontend/src/components/metrics-panel.tsx`: shows latency and pipeline metrics.

## Current State

This is early software. It works end to end, but the goal is still to keep the core readable and honest rather than to cover every production edge case.

Good fits today:

- Reading how a real-time voice loop works.
- Forking the system for experiments.
- Trying different provider combinations.
- Inspecting latency and event flow while a session runs.
- Understanding how WebRTC audio has to be shaped for generated speech.

Not the goal today:

- A hosted platform.
- A general-purpose media server.
- A drop-in replacement for mature real-time infrastructure.
- A fully hardened enterprise deployment.

If you want a feature, open an issue with the use case. The direction should come from what people actually try to build with it.

## Contributing

This project is still early.

If you want to add a provider, transport, deployment guide, or larger change to the orchestration layer, please open an issue first so the use case and approach are clear.

Small fixes, focused PRs, and documentation improvements are welcome.
