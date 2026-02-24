.PHONY: run test lint build validate clean cross-compile

VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/coastguard/cli/internal/version.Version=$(VERSION) \
           -X github.com/coastguard/cli/internal/version.Commit=$(COMMIT) \
           -X github.com/coastguard/cli/internal/version.Date=$(DATE)

run:
	@go run -ldflags '$(LDFLAGS)' ./cmd/coastguard

test:
	@echo "Running tests..."
	@go test ./...

lint:
	@echo "Running linter..."
	@golangci-lint run ./...

build:
	@echo "Building binary..."
	@go build -ldflags '$(LDFLAGS)' -o bin/coastguard ./cmd/coastguard

validate:
	@echo "Running full validation..."
	@go vet ./...
	@golangci-lint run ./...
	@go build -ldflags '$(LDFLAGS)' -o /dev/null ./cmd/coastguard
	@go test -short ./...

cross-compile:
	@echo "Cross-compiling for all platforms..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/coastguard-darwin-amd64 ./cmd/coastguard
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o bin/coastguard-darwin-arm64 ./cmd/coastguard
	@CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/coastguard-linux-amd64 ./cmd/coastguard
	@CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o bin/coastguard-linux-arm64 ./cmd/coastguard
	@echo "Binaries written to bin/"

clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@go clean
