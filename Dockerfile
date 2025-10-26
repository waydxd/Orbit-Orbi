# Build stage
FROM golang:1.21-alpine AS builder

# Install dependencies
RUN apk add --no-cache git make protobuf-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o /app/bin/orbi ./cmd/orbi

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/bin/orbi .

# Copy config files
COPY --from=builder /app/configs ./configs

# Set environment variables (can be overridden)
ENV CALENDAR_SERVICE_ADDR=localhost:50051
ENV OPENAI_MODEL=gpt-3.5-turbo

# Run the application
CMD ["./orbi"]
