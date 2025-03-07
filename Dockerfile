# Build stage
FROM golang:1.24.1-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code with proper structure for embedding
COPY . .
RUN mkdir -p cmd/go-media-control/static
RUN cp -r static/* cmd/go-media-control/static/

# Build the WASM application and server
RUN mkdir -p out/web out/static && \
    GOARCH=wasm GOOS=js go build -o out/web/app.wasm ./cmd/go-media-control && \
    go build -o go-media-control ./cmd/go-media-control && \
    cp -r static/* out/static/

# Final stage
FROM alpine:3.19

WORKDIR /app

# Add CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary, WASM, and static files from builder
COPY --from=builder /app/go-media-control .
COPY --from=builder /app/out/web ./out/web
COPY --from=builder /app/out/static ./out/static

# Create a non-root user to run the application
RUN adduser -D -H -h /app appuser && \
    chown -R appuser:appuser /app
USER appuser

# Set environment variables (override these at runtime)
ENV APP_PORT=8080

# Expose the application port
EXPOSE 8080

# Run the application
ENTRYPOINT ["/app/go-media-control"]
