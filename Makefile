.PHONY: build run clean test lint tidy

BINARY := ollacloud
BUILD_DIR := /usr/local/bin
LDFLAGS := -ldflags="-s -w"

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) .

run: build
	$(BUILD_DIR)/$(BINARY) serve

clean:
	rm -f $(BUILD_DIR)/$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy
