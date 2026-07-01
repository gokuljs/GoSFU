#!/usr/bin/env bash
set -euo pipefail

MODEL_DIR="$(cd "$(dirname "$0")/../assets/models" && pwd)"
MODEL_FILE="$MODEL_DIR/silero_vad.onnx"
MODEL_URL="https://github.com/snakers4/silero-vad/raw/master/src/silero_vad/data/silero_vad.onnx"

if [ -f "$MODEL_FILE" ]; then
  echo "Model already exists at $MODEL_FILE"
  exit 0
fi

mkdir -p "$MODEL_DIR"

echo "Downloading Silero VAD model..."
if command -v curl &>/dev/null; then
  curl -L -o "$MODEL_FILE" "$MODEL_URL"
elif command -v wget &>/dev/null; then
  wget -O "$MODEL_FILE" "$MODEL_URL"
else
  echo "Error: curl or wget is required" >&2
  exit 1
fi

echo "Downloaded to $MODEL_FILE"
