.PHONY: build run test clean install release check-fork-ownership build-release update-homebrew-formula verify-release verify-homebrew-formula ci test-windows-installer validate-scoop-manifest validate-winget-manifests

FORMULA_PATH ?= Formula/ocgo.rb

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

verify-release:
	@[ -n "$(TAG)" ] || (echo "Usage: make verify-release TAG=v0.1.0" && exit 1)
	./scripts/verify-release-artifacts.sh "$(TAG)" dist

verify-homebrew-formula:
	@[ -n "$(TAG)" ] || (echo "Usage: make verify-homebrew-formula TAG=v0.1.0 FORMULA_PATH=$(FORMULA_PATH)" && exit 1)
	./scripts/verify-homebrew-formula.sh "$(TAG)" "$(FORMULA_PATH)"

test-windows-installer:
	pwsh -File ./scripts/test-install-windows.ps1

validate-scoop-manifest:
	pwsh -Command "Get-Content ./packaging/scoop/ocgo.json | ConvertFrom-Json | Out-Null"

validate-winget-manifests:
	pwsh -Command "$$manifestDir = './packaging/winget/manifests/u/UlrichZogo/OCGO/0.1.0'; if (-not (Test-Path (Join-Path $$manifestDir 'UlrichZogo.OCGO.yaml'))) { throw 'Missing OCGO.yaml' }; if (-not (Test-Path (Join-Path $$manifestDir 'UlrichZogo.OCGO.installer.yaml'))) { throw 'Missing installer.yaml' }; if (-not (Test-Path (Join-Path $$manifestDir 'UlrichZogo.OCGO.locale.en-US.yaml'))) { throw 'Missing locale.yaml' }; if (Get-Command winget -ErrorAction SilentlyContinue) { $$out = winget validate $$manifestDir 2>&1; $$out | Write-Host; $$ec = $$LASTEXITCODE; if ($$ec -ne 0) { if ($$out -match 'succeeded') { Write-Host 'winget validate succeeded with warnings (exit ' $$ec ').'; exit 0 }; exit $$ec }; Write-Host 'winget validate completed.' } else { Write-Host 'winget not available; skipping.' }"
