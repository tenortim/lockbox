VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X github.com/tenortim/lockbox/internal/cmd.version=$(VERSION) \
	-X github.com/tenortim/lockbox/internal/cmd.commit=$(COMMIT) \
	-X github.com/tenortim/lockbox/internal/cmd.date=$(DATE)

DESTDIR ?= $(HOME)/.local/bin

.PHONY: help build build-linux build-windows build-all install test vet check clean release release-snapshot

## help: list available targets
help:
	@grep '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## build: compile the binary for the current platform
build:
	go build -ldflags "$(LDFLAGS)" -o lockbox ./cmd/lockbox/

## build-linux: cross-compile a Linux amd64 binary
build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/lockbox-linux-amd64 ./cmd/lockbox/

## build-windows: cross-compile a Windows amd64 binary
build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/lockbox-windows-amd64.exe ./cmd/lockbox/

## build-all: cross-compile for all supported platforms (output goes to dist/)
build-all: build-linux build-windows

## install: build and install to DESTDIR (default: ~/.local/bin)
install: build
	install -d $(DESTDIR)
	install -m 755 lockbox $(DESTDIR)/lockbox

## test: run all tests
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## check: run vet + tests (use before committing)
check: vet test

## clean: remove build artifacts
clean:
	rm -f lockbox
	rm -rf dist/

## release: create a tagged release and push to GitHub via goreleaser
##   Usage: make release V=0.3.0
##   Set GITHUB_TOKEN in your environment, or gh auth token is used as fallback.
release:
ifndef V
	$(error Usage: make release V=x.y.z)
endif
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: working tree is dirty. Commit or stash changes first."; \
		exit 1; \
	fi
	@if git rev-parse "v$(V)" >/dev/null 2>&1; then \
		echo "Tag v$(V) already exists, skipping creation."; \
	else \
		git tag -a "v$(V)" -m "Release v$(V)"; \
	fi
	git push origin "v$(V)"
	@if [ -z "$$GITHUB_TOKEN" ]; then \
		echo "GITHUB_TOKEN not set, falling back to gh auth token"; \
		GITHUB_TOKEN=$$(gh auth token) goreleaser release --clean; \
	else \
		goreleaser release --clean; \
	fi

## release-snapshot: build release artifacts locally without publishing
release-snapshot:
	goreleaser release --snapshot --clean
