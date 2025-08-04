# Multi-stage build for karmada-dashboard with external karmada-mcp-server integration
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make bash gcc musl-dev curl

# Build karmada-mcp-server from source
WORKDIR /tmp
RUN git clone --depth 1 https://github.com/warjiang/karmada-mcp-server.git
WORKDIR /tmp/karmada-mcp-server
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o karmada-mcp-server ./cmd/karmada-mcp-server

# Build karmada-dashboard
WORKDIR /workspace
COPY . .
RUN make all

# Final stage
FROM alpine:3.18

# Install ca-certificates and curl for health checks
RUN apk add --no-cache ca-certificates curl bash

# Create non-root user
RUN addgroup -g 1001 karmada && \
    adduser -D -u 1001 -G karmada karmada

# Create directories
RUN mkdir -p /app/bin && \
    mkdir -p /app/config && \
    mkdir -p /home/karmada/.kube && \
    chown -R karmada:karmada /app /home/karmada

# Copy binaries from builder stage - use the actual built architecture
COPY --from=builder /tmp/karmada-mcp-server/karmada-mcp-server /app/bin/karmada-mcp-server
COPY --from=builder /workspace/_output/bin/linux/arm64/karmada-dashboard-api /app/bin/karmada-dashboard

# Ensure binaries are executable
RUN chmod +x /app/bin/karmada-mcp-server && \
    chmod +x /app/bin/karmada-dashboard

# Set environment variables
ENV KARMADA_MCP_SERVER_PATH=/app/bin/karmada-mcp-server
ENV KUBECONFIG=/home/karmada/.kube/karmada.config
ENV PATH=/app/bin:$PATH

# Switch to non-root user
USER karmada

# Set working directory
WORKDIR /app

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/api/v1/health || exit 1

# Expose port
EXPOSE 8000

# Default command - starts both dashboard and MCP server
CMD ["sh", "-c", "karmada-mcp-server stdio --karmada-kubeconfig=$KUBECONFIG --karmada-context=karmada-apiserver & karmada-dashboard"]