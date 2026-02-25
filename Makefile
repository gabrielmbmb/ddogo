BINARY=ddogo

.PHONY: run build test tidy fmt lint

run:
	go run ./cmd/$(BINARY) --help

build:
	go build -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	golangci-lint fmt ./...

lint:
	golangci-lint run ./...
