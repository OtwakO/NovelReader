# syntax=docker/dockerfile:1.7

FROM node:alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:alpine AS opencc-build
RUN apk add --no-cache build-base cmake curl python3
COPY build/opencc.env /tmp/opencc.env
RUN . /tmp/opencc.env \
    && curl -fsSL "https://github.com/BYVoid/OpenCC/archive/refs/tags/ver.${OPENCC_VERSION}.tar.gz" -o /tmp/opencc.tar.gz \
    && echo "${OPENCC_SHA256}  /tmp/opencc.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/opencc.tar.gz -C /tmp \
    && cmake -S "/tmp/OpenCC-ver.${OPENCC_VERSION}" -B /tmp/opencc-build \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_INSTALL_PREFIX=/opt/opencc \
        -DBUILD_DOCUMENTATION=OFF \
        -DBUILD_OPENCC_JIEBA_PLUGIN=ON \
        -DENABLE_GTEST=OFF \
    && cmake --build /tmp/opencc-build \
    && cmake --install /tmp/opencc-build

FROM golang:alpine AS backend-build
RUN apk add --no-cache build-base pkgconf
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./
COPY build/opencc.env /tmp/opencc.env
COPY --from=opencc-build /opt/opencc /opt/opencc
# Match compilation flags so the release build can reuse dependencies compiled by tests.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    . /tmp/opencc.env \
    && export PKG_CONFIG_PATH=/opt/opencc/lib/pkgconfig LD_LIBRARY_PATH=/opt/opencc/lib CGO_ENABLED=1 \
    && go test -tags=opencc_native -trimpath \
       -ldflags="-X github.com/otwako/novelreader/internal/chineseconv.EngineVersion=${OPENCC_VERSION}" \
       ./internal/chineseconv ./internal/api \
    && go build -tags=opencc_native -trimpath \
       -ldflags="-s -w -X github.com/otwako/novelreader/internal/chineseconv.EngineVersion=${OPENCC_VERSION}" \
       -o /out/novelreader ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates libstdc++ su-exec tzdata \
    && mkdir -p /app/frontend/dist /data
WORKDIR /app
COPY --from=backend-build /out/novelreader ./novelreader
COPY --from=opencc-build /opt/opencc/lib/libopencc.so* /opt/opencc/lib/
COPY --from=opencc-build /opt/opencc/lib/opencc/plugins /opt/opencc/lib/opencc/plugins
COPY --from=opencc-build /opt/opencc/share/opencc /opt/opencc/share/opencc
COPY --from=frontend-build /src/frontend/dist ./frontend/dist
COPY --chmod=755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
LABEL org.opencontainers.image.source="https://github.com/OtwakO/NovelReader"
ENV LD_LIBRARY_PATH=/opt/opencc/lib \
    PORT=8888 \
    DATA_DIR=/data \
    TZ=UTC
EXPOSE 8888
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8888/api/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
