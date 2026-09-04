FROM ghcr.io/astral-sh/uv:0.11.8 AS uv

FROM python:3.13-slim AS build

ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy

COPY --from=uv /uv /uvx /bin/
WORKDIR /app
COPY pyproject.toml uv.lock ./
# Release builds bypass this stage's cache to resolve current stable Patchright.
RUN uv sync --no-dev --no-install-project --upgrade-package patchright

FROM python:3.13-slim

LABEL org.opencontainers.image.source="https://github.com/OtwakO/NovelReader"
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    WEBVIEW_WORKER_HOST=0.0.0.0 \
    WEBVIEW_BROWSER_MODE=headless

COPY --from=build /app/.venv /app/.venv
RUN /app/.venv/bin/patchright install chrome --no-shell --with-deps \
    && apt-get install -y --no-install-recommends xvfb \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/* /usr/share/doc/* /usr/share/man/* /tmp/* /root/.cache/ms-playwright

WORKDIR /app
RUN useradd --create-home --uid 10001 appuser \
    && mkdir -p /tmp/.X11-unix \
    && chmod 1777 /tmp/.X11-unix \
    && chmod -R a+rX /app/.venv
COPY --chown=10001:10001 browser.py interactive.py runtime.py worker.py ./
USER appuser

EXPOSE 8787
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD ["/app/.venv/bin/python", "-c", "import json,urllib.request; data=json.load(urllib.request.urlopen('http://127.0.0.1:8787/healthz',timeout=2)); raise SystemExit(0 if data.get('ok') else 1)"]
CMD ["/app/.venv/bin/python", "worker.py"]
