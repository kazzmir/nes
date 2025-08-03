# syntax=docker/dockerfile:1
FROM golang:1.24.5-bullseye AS build

RUN apt-get update && apt-get install -y \
    build-essential \
    libsdl2-dev \
    libsdl2-ttf-dev \
    libsdl2-mixer-dev \
    libasound2-dev \
    libxcursor-dev \
    libxinerama-dev \
    libxi-dev \
    libxrandr-dev \
    libxss-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY cmd/ cmd/
COPY lib/ lib/
COPY data/ data/
COPY util/ util/
COPY go.mod go.mod
COPY go.sum go.sum

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build -tags static -ldflags "-s -w" -o nes ./cmd/nes

# Final stage: copy binary to /out
FROM debian:bullseye-slim AS final
COPY --from=build /src/nes /out/nes

CMD ["/bin/sh", "-c", "cp /out/nes /mnt/nes-static"]
