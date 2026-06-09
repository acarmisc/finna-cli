VERSION  ?= dev
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE     ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

LDFLAGS := -s -w \
	-X github.com/acarmisc/finna-cli/internal/version.Version=$(VERSION) \
	-X github.com/acarmisc/finna-cli/internal/version.Commit=$(COMMIT) \
	-X github.com/acarmisc/finna-cli/internal/version.Date=$(DATE)

.PHONY: build test lint generate tidy clean man release-dry snapshot

build:
	go build -ldflags "$(LDFLAGS)" -o finna ./cmd/finna

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

generate:
	go generate ./...

tidy:
	go mod tidy

man: build
	./finna man

clean:
	rm -f finna coverage.out
	rm -rf dist man

# Run goreleaser in snapshot mode (no publish, no tag required).
# Useful for verifying the full build matrix locally.
release-dry:
	goreleaser release --snapshot --clean

snapshot:
	goreleaser release --snapshot --skip=publish --clean
