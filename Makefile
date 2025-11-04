.PHONY: proto build build-core run run-server run-core run-all clean test fmt vet deps install-proto-tools check

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@export PATH=$$PATH:$$(go env GOPATH)/bin && \
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/calendar.proto proto/calendar_data.proto
	@echo "Protobuf code generated successfully"

# Build the Orbi Agent binary
build:
	@echo "Building Orbi Agent..."
	@go build -o bin/orbi ./cmd/orbi
	@echo "Build complete: bin/orbi"

# Build the Core CLI binary
build-core:
	@echo "Building Core CLI..."
	@go build -o bin/core ./cmd/core
	@echo "Build complete: bin/core"

# Build both Orbi and Core
build-all: build build-core
	@echo "All builds complete: bin/orbi and bin/core"

# Run the Orbi Agent in interactive mode (default)
run: build
	@echo "Starting Orbi Agent (interactive mode)..."
	@AGENT_MODE=interactive ./bin/orbi

# Run the Orbi Agent in server-only mode
run-server: build
	@echo "Starting Orbi Agent (server mode)..."
	@AGENT_MODE=server ./bin/orbi

# Run the Core CLI (requires Agent running on localhost:50052)
run-core: build-core
	@echo "Starting Core CLI..."
	@./bin/core

# Run both Agent (server mode) and Core CLI (requires two terminals or background process)
run-all: build build-core
	@echo "To run Agent and Core together:"
	@echo "  Terminal 1: make run-server"
	@echo "  Terminal 2: make run-core"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f proto/*.pb.go
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	@go vet ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Install protoc plugins
install-proto-tools:
	@echo "Installing protoc tools..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Protoc tools installed"

# Run all checks
check: fmt vet test
	@echo "All checks passed"
