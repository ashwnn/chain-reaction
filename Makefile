APP_NAME := chain-reaction
PKG := github.com/ashwnn/chain-reaction

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: build test tidy run

build:
	go build -ldflags "-X $(PKG)/internal/buildinfo.Version=$(VERSION) -X $(PKG)/internal/buildinfo.Commit=$(COMMIT) -X $(PKG)/internal/buildinfo.Date=$(DATE)" -o bin/$(APP_NAME) ./cmd/chain-reaction

test:
	go test ./...

tidy:
	go mod tidy

run:
	go run ./cmd/chain-reaction scan
