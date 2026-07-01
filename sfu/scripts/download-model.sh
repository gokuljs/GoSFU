#!/usr/bin/env bash
set -euo pipefail

MODEL_DIR="$(cd "$(dirname "$0")/../assets/models" && pwd)"
MODEL_FILE="$MODEL_DIR/silero_vad.onnx"
MODEL_URL="https://raw.githubusercontent.com/snakers4/silero-vad/master/src/silero_vad/data/silero_vad.onnx"
MIN_MODEL_SIZE_BYTES=1000000

if [ -f "$MODEL_FILE" ]; then
  echo "Model already exists at $MODEL_FILE"
  exit 0
fi

mkdir -p "$MODEL_DIR"
TMP_MODEL_FILE="$(mktemp "$MODEL_DIR/silero_vad.onnx.XXXXXX")"
trap 'rm -f "$TMP_MODEL_FILE"' EXIT

echo "Downloading Silero VAD ONNX model..."
echo "This is the platform-independent model file, not the ONNX Runtime library."
echo "Install ONNX Runtime separately for your OS before running the server."

if command -v curl &>/dev/null; then
  curl -fL --retry 3 -o "$TMP_MODEL_FILE" "$MODEL_URL"
elif command -v wget &>/dev/null; then
  wget -O "$TMP_MODEL_FILE" "$MODEL_URL"
else
  echo "Error: curl or wget is required" >&2
  exit 1
fi

MODEL_SIZE="$(wc -c < "$TMP_MODEL_FILE" | tr -d ' ')"
if [ "$MODEL_SIZE" -lt "$MIN_MODEL_SIZE_BYTES" ]; then
  echo "Error: downloaded file is too small to be the Silero VAD model ($MODEL_SIZE bytes)" >&2
  echo "Check your network connection and retry." >&2
  exit 1
fi

mv "$TMP_MODEL_FILE" "$MODEL_FILE"
trap - EXIT

echo "Downloaded to $MODEL_FILE"
