VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w \
	-X github.com/ojuschugh1/aura/internal/cli.Version=$(VERSION) \
	-X github.com/ojuschugh1/aura/internal/cli.BuildDate=$(DATE) \
	-X github.com/ojuschugh1/aura/internal/cli.Commit=$(COMMIT)

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

.PHONY: build release clean test lint

build:
	go build -ldflags "$(LDFLAGS)" -o bin/aura ./cmd/aura

release:
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		ext=""; \
		if [ "$$GOOS" = "windows" ]; then ext=".exe"; fi; \
		out="dist/aura-$$GOOS-$$GOARCH$$ext"; \
		echo "Building $$out..."; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH \
			go build -ldflags "$(LDFLAGS)" -o $$out ./cmd/aura; \
		sha256sum $$out > $$out.sha256; \
	done

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin/ dist/
