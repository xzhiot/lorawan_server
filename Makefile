.PHONY: help build run stop clean test dev setup

help:
	@echo "LoRaWAN Server Pro - Available commands:"
	@echo ""
	@echo "  make setup       - Initial project setup"
	@echo "  make build       - Build all services"
	@echo "  make run         - Run all services"
	@echo "  make dev         - Run in development mode"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make stop        - Stop all services"

setup:
	@./setup.sh

build:
	@echo "Building services..."
	@go build -o bin/gateway-bridge ./cmd/gateway-bridge
	@go build -o bin/network-server ./cmd/network-server
	@go build -o bin/application-server ./cmd/application-server
	@echo "✓ Build complete"

run:
	@echo "Starting services..."
	@docker-compose up -d
	@echo "✓ Services started"
	@echo "Web UI: http://localhost:8098"

dev:
	@cd scripts && ./start_minimal.sh

test:
	@go test -v ./...

clean:
	@rm -rf bin/
	@docker-compose down -v
	@echo "✓ Clean complete"

stop:
	@docker-compose down
	@echo "✓ Services stopped"
