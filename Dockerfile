# Use the official Go image as build environment
FROM golang:1.21.6 AS builder

# Set metadata labels
LABEL maintainer="Literary Lions Team"
LABEL description="A web forum for book lovers to discuss literature"
LABEL version="1.0"

# Install git for go modules
RUN apt-get update && apt-get upgrade -y && apt-get install -y git && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 go build -o main .

# Use Debian slim for final image (compatible with glibc binary)
FROM debian:bookworm-slim

# Set metadata labels for final image
LABEL maintainer="Literary Lions Team"
LABEL description="Literary Lions Forum - Production Container"
LABEL version="1.0"

# Install necessary packages
RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates sqlite3 wget && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN groupadd -g 1001 appgroup && \
    useradd -r -u 1001 -g appgroup appuser

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/main .

# Copy templates and database schema
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/database/schema.sql ./database/

# Create data and uploads directories
RUN mkdir -p /app/data /app/uploads && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port 8080
EXPOSE 8080

# Set environment variables
ENV PORT=8080
ENV DB_PATH=/app/data/forum.db

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run the application
CMD ["./main"]