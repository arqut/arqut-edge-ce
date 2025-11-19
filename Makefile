.PHONY: help build build-ui build-go clean run dev test install-ui

# Default target
help:
	@echo "Arqut Edge CE - Build Targets"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          - Build both UI and Go binary"
	@echo "  build-ui       - Build UI only (production)"
	@echo "  build-go       - Build Go binary only"
	@echo "  install-ui     - Install UI dependencies"
	@echo "  dev-ui         - Run UI dev server on :9000"
	@echo "  clean          - Clean build artifacts"
	@echo "  run            - Build and run the server"
	@echo "  test           - Run tests"
	@echo ""

# Build everything (UI + Go)
build: build-ui build-go
	@echo "✓ Build complete: bin/arqut-edge-ce-app"

# Build UI for production
build-ui:
	@echo "Building UI..."
	cd ui && npm run build
	@echo "✓ UI built: ui/dist/spa/"

# Build Go binary with embedded UI
build-go:
	@echo "Building Go binary..."
	go build -o bin/arqut-edge-ce-app ./cmd/arqut-edge-ce
	@echo "✓ Go binary built: bin/arqut-edge-ce-app"

# Install UI dependencies
install-ui:
	@echo "Installing UI dependencies..."
	cd ui && npm install
	@echo "✓ UI dependencies installed"

# Run UI dev server (proxies API to :3030)
dev-ui:
	@echo "Starting UI dev server on http://localhost:9000"
	@echo "API requests will be proxied to http://localhost:3030"
	cd ui && npm run dev

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/arqut-edge-ce-app
	rm -rf ui/dist
	rm -rf ui/.quasar
	@echo "✓ Clean complete"

# Build and run with default config
run: build
	@echo "Starting Arqut Edge CE..."
	@echo "Server will run on http://localhost:3030"
	ARQUT_API_KEY=test \
	SERVER_ADDR=:3030 \
	DB_PATH=/tmp/edge.db \
	./bin/arqut-edge-ce-app

# Run tests
test:
	@echo "Running Go tests..."
	go test ./...
	@echo "✓ Tests complete"

# Quick rebuild (no clean)
rebuild: build-ui build-go
	@echo "✓ Rebuild complete"
