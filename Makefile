.PHONY: build run test clean install release check-fork-ownership build-release update-homebrew-formula verify-release verify-homebrew-formula ci test-windows-installer validate-scoop-manifest validate-winget-manifests e2e-smoke real-daemon-smoke release-install-smoke release-install-smoke-build

FORMULA_PATH ?= Formula/ocgo.rb

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS ?= -s -w -X main.version=$(VERSION) -X ocgo/internal/buildinfo.Version=$(VERSION) -X ocgo/internal/buildinfo.Commit=$(COMMIT) -X ocgo/internal/buildinfo.Date=$(DATE)
EXE := $(shell go env GOEXE)
OCGO_BIN := bin/ocgo$(EXE)
GOBIN := $(shell go env GOBIN)
GOPATH := $(shell go env GOPATH)
INSTALL_DIR := $(if $(GOBIN),$(GOBIN),$(GOPATH)/bin)

build:
	go build -ldflags "$(LDFLAGS)" -o $(OCGO_BIN) ./cmd/ocgo

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
	pwsh -Command "$$manifestDir = './packaging/winget/manifests/u/UlrichZogo/OCGO/0.1.0'; if (-not (Test-Path (Join-Path $$manifestDir 'UlrichZogo.OCGO.yaml'))) { throw 'Missing OCGO.yaml' }; if (-not (Test-Path (Join-Path $$manifestDir 'UlrichZogo.OCGO.installer.yaml'))) { throw 'Missing installer.yaml' }; if (-not (Test-Path (Join-Path $$manifestDir 'UlrichZogo.OCGO.locale.en-US.yaml'))) { throw 'Missing locale.yaml' }; if (Get-Command winget -ErrorAction SilentlyContinue) { $$out = winget validate $$manifestDir 2>&1; $$text = ($$out | Out-String); Write-Host $$text; $$ec = $$LASTEXITCODE; if ($$ec -eq 0) { Write-Host 'winget validate completed.'; exit 0 }; $$hasValidationSuccess = ($$text -match 'Manifest validation succeeded') -or ($$text -match 'Validation succeeded') -or ($$text -match 'succeeded with warnings'); $$hasKnownSchemaWarning = ($$text -match 'Schema header not found'); $$hasHardError = ($$text -match '(?i)\\berror\\b') -or ($$text -match '(?i)\\bfailed\\b') -or ($$text -match '(?i)\\binvalid\\b'); if ($$hasValidationSuccess -and $$hasKnownSchemaWarning -and -not $$hasHardError) { Write-Host 'winget validate returned non-zero but only known schema-header warning was detected.'; exit 0 }; Write-Error \"winget validate failed with exit code $$ec.\"; exit $$ec } else { Write-Host 'winget not available; skipping.' }"

e2e-smoke:
	go test ./internal/e2e -run E2E -v

.PHONY: real-daemon-smoke
real-daemon-smoke:
	OCGO_E2E_REAL_DAEMON=1 go test ./internal/e2e -run RealDaemon -v

.PHONY: release-install-smoke
release-install-smoke:
	./scripts/smoke-release-install.sh --dist dist

.PHONY: release-install-smoke-build
release-install-smoke-build:
	./scripts/build-release-artifacts.sh v0.0.0-smoke
	./scripts/smoke-release-install.sh --dist dist --version v0.0.0-smoke
