# MRTI Agent — build & tooling
# Requires: Go 1.22+, protoc + protoc-gen-go + protoc-gen-go-grpc (for `make proto`)

VERSION ?= 0.1.0-dev
LDFLAGS := -s -w -X github.com/jromanMRT/mrti-agent/internal/agent.Version=$(VERSION)
PKG     := github.com/jromanMRT/mrti-agent
BIN     := mrti-agent

.PHONY: all build build-linux build-windows build-plugins proto tidy run clean test vet install-service

all: build

## Build the agent for the host platform
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BIN) ./cmd/mrti-agent

## Cross-compile for Linux amd64
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/linux-amd64/$(BIN) ./cmd/mrti-agent

## Cross-compile for Windows amd64
build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/windows-amd64/$(BIN).exe ./cmd/mrti-agent

## Build the reference ping plugin into the plugins directory
build-plugins:
	go build -trimpath -o plugins/ping ./plugins/example-ping

## Regenerate gRPC stubs from proto/collector.proto
proto:
	protoc --proto_path=proto \
		--go_out=. --go_opt=module=$(PKG) \
		--go-grpc_out=. --go-grpc_opt=module=$(PKG) \
		proto/collector.proto

## Resolve/verify dependencies
tidy:
	go mod tidy

## Run in the foreground with console logging
run: build
	./bin/$(BIN) -foreground -config config.yaml

vet:
	go vet ./...

test:
	go test ./...

## Install the systemd service (Linux; run as root)
install-service: build-linux
	./scripts/install-linux.sh

clean:
	rm -rf bin dist
