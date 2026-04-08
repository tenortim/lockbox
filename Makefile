VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X github.com/tenortim/lockbox/internal/cmd.version=$(VERSION) \
	-X github.com/tenortim/lockbox/internal/cmd.commit=$(COMMIT) \
	-X github.com/tenortim/lockbox/internal/cmd.date=$(DATE)

.PHONY: build test vet check clean release release-snapshot

## build: compile the binary for the current platform
build:
	go build -ldflags "$(LDFLAGS)" -o lockbox ./cmd/lockbox/

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
##   Usage: make release V=0.2.0
release:
ifndef V
	$(error Usage: make release V=x.y.z)
endif
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: working tree is dirty. Commit or stash changes first."; \
		exit 1; \
	fi
	git tag -a "v$(V)" -m "Release v$(V)"
	git push origin "v$(V)"
	goreleaser release --clean

## release-snapshot: build release artifacts locally without publishing
release-snapshot:
	goreleaser release --snapshot --clean
