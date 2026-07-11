# Headless Load Testing

This guide covers load testing the SFU without a browser. Each virtual client is a Go process that creates its own room, joins over WebRTC, and pumps synthetic microphone audio into the agent pipeline.

The browser UI (`sfu-frontend`) is optional — use it only for manual smoke tests.

## Prerequisites

- Go 1.26+
- SFU built from `sfu/`

```bash
cd sfu
go build -o sfu ./cmd/sfu
go build -o loadtest ./cmd/loadtest
```

## Quick start (3 clients)

**Terminal 1 — start SFU with stub providers (no external API calls):**

```bash
cd sfu
./sfu --load-test --port 8080
```

`--load-test` forces stub LLM/STT/TTS/VAD providers for every room join. Without it, joins use real OpenAI/Deepgram/Rime/Silero credentials.

**Terminal 2 — run headless clients:**

```bash
cd sfu
./loadtest --clients 3 --duration 20s --turns 1
```

Each client:

1. `POST /room/create` — one room per client (rooms allow one active participant)
2. `GET /ice-config`
3. Creates a Pion `PeerConnection`, adds an Opus mic track
4. `POST /room/{id}/join` with a complete SDP offer
5. Pumps fake audio until `--duration` elapses or `--turns` final agent transcripts are seen
6. `DELETE /room/{id}` on exit

## 100-client run

```bash
# Terminal 1
cd sfu
./sfu --load-test --port 8080

# Terminal 2
cd sfu
./loadtest --clients 100 --duration 60s --stagger 50ms
```

Tune `--stagger` to avoid thundering-herd joins. Increase `--duration` to keep sessions alive longer.

For a one-liner without building binaries:

```bash
# Terminal 1
cd sfu && go run ./cmd/sfu --load-test --port 8080

# Terminal 2
cd sfu && go run ./cmd/loadtest --clients 100 --duration 60s --stagger 50ms
```

## Fake audio

By default each client injects a **440 Hz sine wave** at ~35% amplitude as 48 kHz mono Opus frames (20 ms per packet). This is loud enough for stub VAD (RMS threshold 500) and keeps stub STT fed with continuous speech frames.

Override with a WAV file:

```bash
./loadtest --clients 10 --wav /path/to/sample.wav
```

Requirements: mono, 16-bit PCM. Any sample rate is resampled to 48 kHz. The file loops for the session duration.

TTS output is generated server-side by the stub TTS plugin (~300 ms silence per turn). The load test client does not synthesize TTS — it only fakes the **microphone input**.

## CLI reference

### `sfu`

| Flag | Default | Description |
|------|---------|-------------|
| `--load-test` | `false` | Use stub AI providers (no external API calls) |
| `--port` | `8080` | HTTP port |
| `--env` | `local` | Log environment |

### `loadtest`

| Flag | Default | Description |
|------|---------|-------------|
| `--base-url` | `http://localhost:8080` | SFU URL (`SFU_URL` env override) |
| `--clients` | `1` | Concurrent headless clients |
| `--duration` | `30s` | Per-client connection time |
| `--turns` | `0` | Wait for N final agent transcript turns (`0` = duration only) |
| `--stagger` | `100ms` | Delay between client spawns |
| `--wav` | | Optional WAV mic source |
| `--system-prompt` | | Optional join prompt |
| `--wait-server` | `10s` | Startup wait for SFU health |

## Browser smoke test (optional)

```bash
cd sfu-frontend
bun install
bun run dev
```

Open the UI, create a room, and click **Start Session** to verify end-to-end A/V manually. This is not used for load testing.

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| `join: HTTP 409` | Two clients tried the same room (each client creates its own) |
| `timed out waiting for connection` | ICE/TURN issue; check `STUN_URLS` / `TURN_URLS` in `.env` |
| `saw 0 agent turns` with `--turns` | Audio too quiet, or server not in `--load-test` mode with missing API keys |
| High CPU at 100 clients | Expected — each room runs a full agent pipeline; scale horizontally with `NODE_ID` + Redis |

## Architecture note

```
loadtest client                    SFU (--load-test)
───────────────                    ────────────────
sine/WAV → Opus mic track  ──►    stub VAD → stub STT → stub LLM → stub TTS
       ◄── agent Opus track        (WebRTC answer + agent audio)
WS /room/{id}/stream (optional)   transcript/debug/metrics
```

One room per client. Redis (`REDIS_URL`) is optional for single-node load tests.
