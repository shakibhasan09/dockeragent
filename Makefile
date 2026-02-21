APP_NAME := dockeragent
BUILD_DIR := ./dist

.PHONY: build run vet test clean

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/dockeragent

run:
	go run ./cmd/dockeragent

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)
