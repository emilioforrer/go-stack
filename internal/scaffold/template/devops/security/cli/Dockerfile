# Multi-stage build for SonarQube CLI tool
FROM golang:1.25-alpine AS builder

# Set working directory
WORKDIR /app

# Install git (needed for go mod)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sonar-cli main.go

# Final stage - minimal runtime image
FROM alpine:3.20

# Install ca-certificates and su-exec for user switching, then create non-root user
RUN apk --no-cache add ca-certificates su-exec && \
    addgroup -g 1000 -S sonar && \
    adduser -u 1000 -S sonar -G sonar

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/sonar-cli .

# Copy the entrypoint script
COPY entrypoint.sh .

# Create reports directory and set permissions
RUN mkdir -p /app/reports && \
    chown -R sonar:sonar /app && \
    chmod +x entrypoint.sh

# Set the entrypoint (runs as root to fix permissions, then switches to sonar user)
ENTRYPOINT ["./entrypoint.sh"]

USER sonar


# Default command shows help
CMD []
