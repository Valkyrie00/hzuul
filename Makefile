APP_NAME := hzuul
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build run clean tidy lint

build:
	go build $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/hzuul

run: build
	./bin/$(APP_NAME)

tidy:
	go mod tidy

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...
