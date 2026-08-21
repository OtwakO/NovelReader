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
RUN apk add --no-cache ca-certificates su-exec tzdata \
    && mkdir -p /app/frontend/dist /data
WORKDIR /app
COPY --from=backend-build /out/novelreader ./novelreader
COPY --from=frontend-build /src/frontend/dist ./frontend/dist
COPY --chmod=755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
LABEL org.opencontainers.image.source="https://github.com/OtwakO/NovelReader"
ENV PORT=8888 \
    DATA_DIR=/data \
    TZ=UTC
EXPOSE 8888
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8888/api/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
