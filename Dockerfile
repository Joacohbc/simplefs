# ==============================================================================
# SimpleFS Multi-Stage Dockerfile
# Stage 1: Frontend Asset Compilation (Node.js + pnpm)
# Stage 2: Backend Compilation (Go 1.22)
# Stage 3: Minimal and Secure Runtime Image (Alpine Linux)
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Frontend Builder
# ------------------------------------------------------------------------------
FROM node:20-alpine AS frontend-builder
WORKDIR /app

# Enable corepack and activate pnpm matching project specs
RUN corepack enable && corepack prepare pnpm@9 --activate

# Copy dependency specifications and lockfile for caching layer
COPY package.json pnpm-lock.yaml ./

# Install dependencies deterministically
RUN pnpm install --frozen-lockfile

# Copy required assets and configuration for compilation
COPY tailwind.config.js ./
COPY web/ ./web/

# Compile frontend vendor bundle and minified Tailwind CSS
RUN pnpm run build:vendor && pnpm run build:css

# ------------------------------------------------------------------------------
# Stage 2: Go Backend Builder
# ------------------------------------------------------------------------------
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app

# Copy Go module definitions and download dependencies
COPY go.mod ./
RUN go mod download

# Copy backend source code and web templates
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/templates/ ./web/templates/
COPY web/embed.go ./web/embed.go

# Copy compiled frontend assets from Stage 1 into the static directory for go:embed
COPY --from=frontend-builder /app/web/static/ ./web/static/

# Compile a statically-linked standalone binary without CGO
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/bin/simplefs \
    ./cmd/simplefs/main.go

# ------------------------------------------------------------------------------
# Stage 3: Final Secure Runtime Image
# ------------------------------------------------------------------------------
FROM alpine:3.20 AS runtime

# Install CA certificates for secure connections and tzdata for timezone support
RUN apk --no-cache add ca-certificates tzdata

# Create dedicated unprivileged group and user (UID/GID 10001)
RUN addgroup -g 10001 -S simplefs && \
    adduser -u 10001 -S simplefs -G simplefs -h /home/simplefs

# Create storage volume directory and assign ownership
RUN mkdir -p /data && \
    chown -R simplefs:simplefs /data

# Copy binary from the backend-builder stage
COPY --from=backend-builder /app/bin/simplefs /usr/local/bin/simplefs

# Default environment configuration
ENV PORT=8080 \
    STORAGE_DIR=/data

# Switch to the non-root user
USER simplefs:simplefs
WORKDIR /data

# Expose default HTTP port
EXPOSE 8080

# Expose persistent data volume
VOLUME ["/data"]

# Define entrypoint and default command
ENTRYPOINT ["/usr/local/bin/simplefs"]
CMD []
