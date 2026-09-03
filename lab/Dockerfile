FROM golang:1.27.0-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /headscale-tui ./cmd/headscale-tui

FROM headscale/headscale:0.29.3-debug

USER root
COPY --from=build /headscale-tui /usr/local/bin/headscale-tui
