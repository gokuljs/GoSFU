# GoSFU

A forkable WebRTC voice agent server in Go.

Handles the audio transport, turn orchestration, pacing, and barge-in so you can focus on your agent logic.

[![license](https://img.shields.io/badge/license-MIT-2ea44f)](LICENSE) [![version](https://img.shields.io/badge/version-v0.1.0-orange)](https://github.com/gokuljs/goSfu) [![demo](https://img.shields.io/badge/demo-YouTube-FF0000?logo=youtube&logoColor=white)](https://youtu.be/-j_NcqJXxuA)

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

**Requirements:** Go 1.26+, Bun 1.3+. Redis is optional — leave `REDIS_URL` unset to use the in-memory fallback.

### Get API keys

The demo room uses these providers and requires keys from each:

| Role | Provider | Get a key |
|------|----------|-----------|
| LLM | OpenAI | [platform.openai.com](https://platform.openai.com/api-keys) |
| Speech-to-text | Deepgram | [console.deepgram.com](https://console.deepgram.com/) |
| Text-to-speech | Rime | [rime.ai](https://rime.ai) |

Stub providers exist in the repo for provider development and custom agent wiring, but the checked-in room demo uses OpenAI, Deepgram, Rime, and Silero VAD.

To use a different vendor, add a provider implementation under `sfu/plugins/`, register it with the matching plugin package, and wire it into the agent config. Provider names only work after their plugin exists.

### Install ONNX Runtime

Voice activity detection uses [Silero VAD](https://github.com/snakers4/silero-vad) via ONNX Runtime.

There are two separate pieces:

- **ONNX Runtime** is platform-specific. Install the build for your OS and CPU.
- **`silero_vad.onnx`** is the model file. It is the same on macOS and Linux, and `./scripts/download-model.sh` downloads it later.

macOS:

```bash
brew install onnxruntime
```

Linux:

```bash
ORT_VERSION=1.18.1
ORT_ARCH=linux-x64 # use linux-aarch64 for ARM64 Linux

curl -LO "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-${ORT_ARCH}-${ORT_VERSION}.tgz"
tar -xzf "onnxruntime-${ORT_ARCH}-${ORT_VERSION}.tgz"

sudo mkdir -p /usr/local/include/onnxruntime /usr/local/lib
sudo cp "onnxruntime-${ORT_ARCH}-${ORT_VERSION}"/include/*.h /usr/local/include/onnxruntime/
sudo cp "onnxruntime-${ORT_ARCH}-${ORT_VERSION}"/lib/libonnxruntime.so* /usr/local/lib/
sudo ldconfig
```

### Backend

```bash
cd sfu
./scripts/download-model.sh
go mod tidy
cp .env.example .env
```

Edit `sfu/.env` — add your `OPENAI_API_KEY`, `DEEPGRAM_API_KEY`, and `RIME_API_KEY`. See `sfu/.env.example` for the full list.

```bash
go run ./cmd/sfu
```

### Frontend (new terminal)

```bash
cd sfu-frontend
cp .env.example .env
bun install
bun run dev
```

Open `http://localhost:3000`, click connect, and allow microphone access.

## TURN Server

GoSFU can run with STUN only for simple local networks, but production WebRTC deployments should provide a TURN server. TURN gives browsers a relay path when direct ICE candidates fail because of strict NATs, firewalls, or container networking.

### Install Docker

If Docker is not installed, install [Docker Desktop](https://www.docker.com/products/docker-desktop/) and confirm it is running:

```bash
docker --version
docker ps
```

### Local TURN Test

Use this only when testing everything on one machine: browser, SFU, and coturn. The `--allow-loopback-peers` flag is needed because local testing relays to `127.0.0.1`; do not use that flag as a default production setting.

Choose a username and password, then use the same values in both the Docker command and `sfu/.env`.

```bash
docker rm -f coturn 2>/dev/null

docker run -d --name coturn \
  -p 3478:3478/udp \
  -p 3478:3478/tcp \
  -p 49160-49200:49160-49200/udp \
  coturn/coturn:latest \
  -n --log-file=stdout --verbose \
  --listening-port=3478 \
  --listening-ip=0.0.0.0 \
  --fingerprint \
  --lt-cred-mech \
  --user=gosfu:change-me \
  --realm=localhost \
  --external-ip=127.0.0.1 \
  --relay-ip=127.0.0.1 \
  --min-port=49160 \
  --max-port=49200 \
  --allow-loopback-peers
```

Configure `sfu/.env` with matching credentials. Leave `STUN_URLS` unset to keep the default STUN list, or set your own STUN URLs explicitly.

```env
TURN_URLS=turn:localhost:3478?transport=udp,turn:localhost:3478?transport=tcp
TURN_USERNAME=gosfu
TURN_CREDENTIAL=change-me
```

Restart the SFU after changing `.env`, then verify the backend returns TURN:

```bash
curl -s http://localhost:8080/ice-config | python3 -m json.tool
```

You should see the default `stun:` URLs plus the `turn:` URLs with the same username/credential. Open `chrome://webrtc-internals`, join a fresh room, and confirm TURN candidates appear as `typ relay`. coturn logs should show allocate/session activity when the relay path is used:

```bash
docker logs -f coturn
```

### Production TURN

For production, run coturn on a host with a stable public IP or DNS name. Open UDP/TCP `3478` and the relay UDP range you choose, for example `49160-49200`.

Replace `YOUR_PUBLIC_IP` with the public IP of the TURN host and choose a strong credential:

```bash
docker rm -f coturn 2>/dev/null

docker run -d --name coturn --restart unless-stopped \
  -p 3478:3478/udp \
  -p 3478:3478/tcp \
  -p 49160-49200:49160-49200/udp \
  coturn/coturn:latest \
  -n --log-file=stdout \
  --listening-port=3478 \
  --listening-ip=0.0.0.0 \
  --fingerprint \
  --lt-cred-mech \
  --user=gosfu:replace-with-a-strong-password \
  --realm=turn.example.com \
  --external-ip=YOUR_PUBLIC_IP \
  --min-port=49160 \
  --max-port=49200
```

Configure the SFU with the same username and password. Keep `STUN_URLS` unset to use the default STUN list, or set it to your own STUN service.

```env
TURN_URLS=turn:turn.example.com:3478?transport=udp,turn:turn.example.com:3478?transport=tcp
TURN_USERNAME=gosfu
TURN_CREDENTIAL=replace-with-a-strong-password
```
## Contributing

The direction of this project should come from what people actually try to build with it.

If you want a feature or hit a limitation, open an issue with the use case. Provider implementations, transport options, deployment guides, and orchestration changes are all welcome; start with an issue so the approach is clear.

Small fixes and documentation improvements can go straight to a PR.

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, checks, and PR guidelines.
