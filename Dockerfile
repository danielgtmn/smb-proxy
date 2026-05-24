# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS builder

ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/smb-proxy ./cmd/smb-proxy

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        cifs-utils \
        samba \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/smb-proxy /usr/local/bin/smb-proxy

RUN mkdir -p /mnt/remote /run/smb-proxy

EXPOSE 445

ENTRYPOINT ["/usr/local/bin/smb-proxy"]
