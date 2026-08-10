# syntax=docker/dockerfile:1.7

FROM node:alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ ./
RUN npm test && npm run build

FROM golang:alpine AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/novelreader ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 app \
    && adduser -D -H -u 10001 -G app app \
    && mkdir -p /app/frontend/dist /data \
    && chown -R 10001:10001 /app /data
WORKDIR /app
COPY --from=backend-build --chown=10001:10001 /out/novelreader ./novelreader
COPY --from=frontend-build --chown=10001:10001 /src/frontend/dist ./frontend/dist
ENV PORT=8888 \
    DATA_DIR=/data \
    TZ=UTC
USER 10001:10001
EXPOSE 8888
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8888/api/healthz || exit 1
ENTRYPOINT ["/app/novelreader"]
