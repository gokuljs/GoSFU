# GoSFU

> Work in progress — a better README is coming later.

This is a real-time WebRTC voice agent system built in Go and React. The browser streams microphone audio to a Go SFU backend, where it runs through voice activity detection, speech-to-text, an LLM, and text-to-speech, then sends the agent's voice back to the browser in real time.

A big part of the work is the media pipeline: decoding browser Opus audio into PCM, converting between 48kHz WebRTC audio and 16kHz model audio, pacing bursty AI-generated speech into smooth 20ms frames, and handling buffering and backpressure so playback does not stutter.

It also includes a live debug console that shows transcripts, connection states, pipeline events, latency metrics, and quota usage, so the voice agent can be inspected while the call is running.

## Demo Video

[![Watch the GoSFU demo on YouTube](https://img.shields.io/badge/Watch%20demo-YouTube-red?style=for-the-badge&logo=youtube&logoColor=white)](https://youtu.be/ZRbSO5N5Q8o?si=rxhzvA9pHx142EFH)


## Prerequisites

- **Node.js** 20+ (for the browser client)
- **Go** 1.26+ (for the SFU server)
- **Redis** 7+ (room state and live event streaming)
- **ONNX Runtime** + Silero VAD model (for voice activity detection)

## First-time setup

### 1. Browser client

```bash
cd sfu-frontend
npm install
```

Runs on **http://localhost:3000** (Vite). The client talks to the server at `http://localhost:8080`.

### 2. Server

```bash
cd sfu
go mod download
```

### 3. API keys & env

Copy the example env file and fill in your keys:

```bash
cd sfu
cp .env.example .env
```

Edit `sfu/.env` — never commit this file.

| Variable | Required for |
|----------|----------------|
| `OPENAI_API_KEY` | LLM (OpenAI) |
| `DEEPGRAM_API_KEY` | Speech-to-text |
| `RIME_API_KEY` | Text-to-speech |
| `ONNXRUNTIME_LIB_PATH` | Silero VAD — path to `libonnxruntime` on your machine |
| `SILERO_MODEL_PATH` | Silero VAD — path to `silero_vad.onnx` (see `sfu/assets/models/`) |
| `REDIS_URL` | Redis connection URL (default: `redis://localhost:6379`) |
| `NODE_ID` | Unique ID for this SFU instance (default: `local`) |
| `SESSION_MAX_DURATION` | Max room session length, e.g. `30m` (default: `30m`) |

Optional overrides (defaults are fine for local dev):

```bash
PORT=8080
ENV=local
```

See `sfu/.env.example` for Deepgram / Rime tuning options.

### 4. Redis

Install and start Redis before running the SFU server.

**macOS (Homebrew)**

```bash
brew install redis
brew services start redis
```

Verify Redis is running:

```bash
redis-cli ping
# PONG
```

## Run

Start **Redis** first, then the **browser client**, then the **server** (three terminals).

**Terminal 1 — Redis** (skip if already running as a service)

```bash
redis-server
```

**Terminal 2 — browser**

```bash
cd sfu-frontend
npm run dev
```

**Terminal 3 — server**

```bash
cd sfu
go run ./cmd/sfu
```

Open **http://localhost:3000**, click connect, and allow mic access when prompted.

## Project layout

```
sfu/            Go SFU + voice agent server (port 8080)
sfu-frontend/   React debug console (port 3000)
```
