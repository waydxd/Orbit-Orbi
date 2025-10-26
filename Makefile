.PHONY: proto build run clean test fmt vet

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@export PATH=$$PATH:$$(go env GOPATH)/bin && \
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/calendar.proto
	@echo "Protobuf code generated successfully"

# Build the Orbi binary
build:
	@echo "Building Orbi..."
	@go build -o bin/orbi ./cmd/orbi
	@echo "Build complete: bin/orbi"

# Run the Orbi chatbot
run: build
	@echo "Starting Orbi chatbot..."
	@./bin/orbi

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
