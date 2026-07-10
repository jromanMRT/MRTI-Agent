# MRTI Agent — build & tooling
# Requires: Go 1.22+, protoc + protoc-gen-go + protoc-gen-go-grpc (for `make proto`)

VERSION ?= 0.1.0-dev
LDFLAGS := -s -w -X github.com/jromanMRT/mrti-agent/internal/agent.Version=$(VERSION)
PKG     := github.com/jromanMRT/mrti-agent
BIN     := mrti-agent

.PHONY: all build build-core build-linux build-windows build-plugins package-windows proto tidy run run-core clean test vet install-service

all: build build-core

## Build the agent for the host platform
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BIN) ./cmd/mrti-agent

## Build the reference Core server for the host platform
build-core:
	go build -trimpath -ldflags "-s -w" -o bin/mrti-core ./cmd/mrti-core

## Cross-compile the agent for Linux amd64
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/linux-amd64/$(BIN) ./cmd/mrti-agent

## Cross-compile agent + core + plugin for Windows amd64
build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/windows-amd64/$(BIN).exe ./cmd/mrti-agent
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/windows-amd64/mrti-core.exe ./cmd/mrti-core
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/windows-amd64/plugins/ping.exe ./plugins/example-ping

## Assemble a ready-to-install Windows zip (agent + core + plugin + config + installer)
package-windows: build-windows
	rm -rf dist/mrti-agent-windows-amd64 dist/mrti-agent-windows-amd64.zip
	mkdir -p dist/mrti-agent-windows-amd64/plugins
	cp dist/windows-amd64/mrti-agent.exe dist/windows-amd64/mrti-core.exe dist/mrti-agent-windows-amd64/
	cp dist/windows-amd64/plugins/ping.exe dist/mrti-agent-windows-amd64/plugins/
	cp config.yaml.example dist/mrti-agent-windows-amd64/config.yaml
	cp packaging/windows/install-windows.ps1 dist/mrti-agent-windows-amd64/install-windows.ps1
	cp packaging/windows/README.txt dist/mrti-agent-windows-amd64/README.txt
	cd dist && (command -v zip >/dev/null 2>&1 && zip -qr mrti-agent-windows-amd64.zip mrti-agent-windows-amd64 || python3 -c "import shutil;shutil.make_archive('mrti-agent-windows-amd64','zip','.','mrti-agent-windows-amd64')")
	@echo "Package: dist/mrti-agent-windows-amd64.zip"

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

## Run the agent in the foreground with console logging
run: build
	./bin/$(BIN) -foreground -config config.yaml

## Run the Core server (dashboard + API + metrics)
run-core: build-core
	./bin/mrti-core -addr :8477 -db core.db

vet:
	go vet ./...

test:
	go test ./...

## Install the systemd service (Linux; run as root)
install-service: build-linux
	./scripts/install-linux.sh

clean:
	rm -rf bin dist
