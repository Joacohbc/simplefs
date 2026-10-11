# syntax=docker/dockerfile:1
# ==============================================================================
# SimpleFS multi-stage build
#   1. frontend: compiles CSS/JS/icons once on the build host (output is arch independent)
#   2. backend:  cross-compiles a static Go binary for the target platform (no QEMU)
#   3. runtime:  scratch image with only the binary (~10 MB)
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: frontend assets
# ------------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend
WORKDIR /app
RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN --mount=type=cache,id=pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

COPY scripts/build-icons.mjs ./scripts/
COPY tailwind.config.js ./
COPY web/src/ ./web/src/
COPY web/templates/ ./web/templates/
COPY web/static/ ./web/static/
COPY internal/ ./internal/
RUN pnpm run build:vendor && pnpm run build:css

# ------------------------------------------------------------------------------
# Stage 2: Go binary, cross-compiled natively on the build host
# ------------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend
WORKDIR /app
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

COPY go.mod ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/embed.go ./web/
COPY web/templates/ ./web/templates/
COPY --from=frontend /app/web/static/ ./web/static/

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
      -trimpath \
      -tags timetzdata \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/simplefs ./cmd/simplefs \
 && mkdir -p /out/data

# ------------------------------------------------------------------------------
# Stage 3: runtime
# ------------------------------------------------------------------------------
FROM scratch AS runtime

COPY --from=backend /out/simplefs /simplefs
COPY --from=backend --chown=10001:10001 /out/data /data

ENV PORT=8080 \
    STORAGE_DIR=/data

USER 10001:10001
WORKDIR /data
EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/simplefs", "healthcheck"]

ENTRYPOINT ["/simplefs"]
