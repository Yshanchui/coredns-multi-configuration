# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install templ CLI
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate templ code
RUN templ generate

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o coredns-manager .

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/coredns-manager .
COPY --from=builder /app/config.yaml .

# Create data directory
RUN mkdir -p /app/data

# Expose port
EXPOSE 8080

# Run
CMD ["./coredns-manager"]
