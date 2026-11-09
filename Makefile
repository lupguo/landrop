MODULE  := github.com/lupguo/landrop
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.Commit=$(COMMIT) -X $(MODULE)/internal/version.Date=$(DATE)
TARGETS := darwin/arm64 darwin/amd64 windows/amd64 linux/amd64 linux/arm64

.PHONY: all web build test dist size e2e

all: web build

web:
	cd web && npm ci && npm run build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/landrop ./cmd/landrop

test:
	go test ./... && cd web && npm test

dist:
	@mkdir -p dist
	@for t in $(TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; ext=""; [ $$os = windows ] && ext=".exe"; \
		echo "building $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/landrop-$$os-$$arch$$ext ./cmd/landrop || exit 1; \
	done

size:
	ls -l dist/

E2E_PORT := 18848

# Builds, starts a throwaway server (temp HOME, never opens a browser) and runs Playwright against it.
e2e: all
	@mkdir -p web/test-results
	@H=$$(mktemp -d); LOG="$$PWD/web/test-results/server.log"; : > "$$LOG"; \
	HOME="$$H" USERPROFILE="$$H" bin/landrop --no-open --port $(E2E_PORT) --lang en > "$$LOG" 2>&1 & \
	PID=$$!; \
	trap 'kill $$PID 2>/dev/null; wait $$PID 2>/dev/null; rm -rf "$$H"' EXIT INT TERM; \
	i=0; until curl -s -o /dev/null http://127.0.0.1:$(E2E_PORT)/; do \
		i=$$((i+1)); [ $$i -gt 100 ] && { echo "server did not start"; cat "$$LOG"; exit 1; }; sleep 0.1; \
	done; \
	cd web && LANDROP_LOG="$$LOG" LANDROP_BASE=http://127.0.0.1:$(E2E_PORT) npx playwright test
