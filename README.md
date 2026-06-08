# GoSFU

> Work in progress — a better README is coming later.

WebRTC SFU with a voice agent debug console (Go server + React browser client).

<img width="1920" height="1080" alt="image" src="https://github.com/user-attachments/assets/1c96b1a9-7ae1-48af-9de0-c27f2eb91661" />


## Prerequisites

- **Node.js** 20+ (for the browser client)
- **Go** 1.26+ (for the SFU server)
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

Optional overrides (defaults are fine for local dev):

```bash
PORT=8080
ENV=local
```

See `sfu/.env.example` for Deepgram / Rime tuning options.

## Run

Start the **browser client first**, then the **server** (two terminals).

**Terminal 1 — browser**

```bash
cd sfu-frontend
npm run dev
```

**Terminal 2 — server**

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
