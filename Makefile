VERSION ?= dev
LDFLAGS := -s -w -X github.com/acarmisc/finna-cli/internal/version.Version=$(VERSION)

.PHONY: build test lint generate tidy clean man

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
