.PHONY: build run test clean install release check-fork-ownership build-release update-homebrew-formula ci

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
EXE := $(shell go env GOEXE)
OCGO_BIN := bin/ocgo$(EXE)
GOBIN := $(shell go env GOBIN)
GOPATH := $(shell go env GOPATH)
INSTALL_DIR := $(if $(GOBIN),$(GOBIN),$(GOPATH)/bin)

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(OCGO_BIN) ./cmd/ocgo

run:
	go run ./cmd/ocgo

test: check-fork-ownership
	go test ./...

ci: check-fork-ownership
	go vet ./...
	go test ./...
	go build ./cmd/ocgo

clean:
	rm -rf bin dist

install: build
	mkdir -p "$(INSTALL_DIR)"
	install -m 0755 "$(OCGO_BIN)" "$(INSTALL_DIR)/ocgo$(EXE)"

check-fork-ownership:
	./scripts/check-fork-ownership.sh

release:
	@[ -n "$(TAG)" ] || (echo "Usage: make release TAG=v0.1.0" && exit 1)
	./scripts/release.sh "$(TAG)"

build-release:
	@[ -n "$(TAG)" ] || (echo "Usage: make build-release TAG=v0.2.0" && exit 1)
	./scripts/build-release-artifacts.sh "$(TAG)"

update-homebrew-formula:
	@[ -n "$(TAG)" ] || (echo "Usage: make update-homebrew-formula TAG=v0.2.0" && exit 1)
	./scripts/update-homebrew-formula.sh "$(TAG)" dist/checksums.txt
