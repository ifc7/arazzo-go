VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

.PHONY: build
build:
	go build -ldflags "\
		-X github.com/shaunhoulihan/arazzo-go/internal.BuildVersion=$(VERSION) \
		-X github.com/shaunhoulihan/arazzo-go/internal.GitCommit=$(GIT_COMMIT) \
		-X github.com/shaunhoulihan/arazzo-go/internal.BuildTime=$(BUILD_TIME)" \
		-o ./bin/arazzo ./cmd/arazzo

.PHONY: install
install: build
	cp ./bin/arazzo $(GOPATH)/bin/arazzo