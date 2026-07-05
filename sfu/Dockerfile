FROM golang:1.26-bookworm AS builder

ARG ORT_VERSION=1.18.1
ARG TARGETARCH

RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    gcc \
    g++ \
    make \
    pkg-config \
    libopus-dev \
    && rm -rf /var/lib/apt/lists/*

RUN case "${TARGETARCH}" in \
      "amd64") ORT_ARCH="linux-x64" ;; \
      "arm64") ORT_ARCH="linux-aarch64" ;; \
      *) echo "Unsupported TARGETARCH=${TARGETARCH}" && exit 1 ;; \
    esac && \
    curl -fL -o /tmp/onnxruntime.tgz \
      "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-${ORT_ARCH}-${ORT_VERSION}.tgz" && \
    tar -xzf /tmp/onnxruntime.tgz -C /tmp && \
    mkdir -p /usr/local/include/onnxruntime /usr/local/lib && \
    cp /tmp/onnxruntime-${ORT_ARCH}-${ORT_VERSION}/include/*.h /usr/local/include/onnxruntime/ && \
    cp /tmp/onnxruntime-${ORT_ARCH}-${ORT_VERSION}/lib/libonnxruntime.so* /usr/local/lib/ && \
    ldconfig && \
    rm -rf /tmp/onnxruntime*

WORKDIR /src/sfu
COPY go.mod go.sum ./
RUN go mod download

RUN mkdir -p assets/models && \
    curl -fL --retry 3 \
      -o assets/models/silero_vad.onnx \
      "https://raw.githubusercontent.com/snakers4/silero-vad/master/src/silero_vad/data/silero_vad.onnx"

ENV CGO_ENABLED=1

FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    libopus0 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /usr/local/lib/libonnxruntime.so* /usr/local/lib/
