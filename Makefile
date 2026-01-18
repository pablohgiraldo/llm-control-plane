SHELL := /bin/sh

APP_NAME := api-gateway
BIN_DIR := bin

.PHONY: all build run test clean fmt vet

all: build

build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/api-gateway

run:
	@echo "Running $(APP_NAME)..."
	@go run ./cmd/api-gateway

test:
	@echo "Running tests..."
	@go test ./...

fmt:
	@echo "Formatting..."
	@gofmt -s -w .

vet:
	@echo "Vet..."
	@go vet ./...

clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)

compose-up:
	@docker compose up -d

compose-down:
	@docker compose down -v

