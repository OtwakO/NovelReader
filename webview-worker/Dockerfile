FROM ghcr.io/astral-sh/uv:0.11.8 AS uv

FROM python:3.13-slim

LABEL org.opencontainers.image.source="https://github.com/OtwakO/NovelReader"
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
    UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    WEBVIEW_WORKER_HOST=0.0.0.0

COPY --from=uv /uv /uvx /bin/
WORKDIR /app
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project \
    && uv run --frozen --no-sync patchright install --with-deps chromium \
    && chmod -R a+rX /ms-playwright /app/.venv

RUN useradd --create-home --uid 10001 appuser
COPY --chown=10001:10001 browser.py interactive.py worker.py ./
USER appuser

EXPOSE 8787
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD ["/app/.venv/bin/python", "-c", "import json,urllib.request; data=json.load(urllib.request.urlopen('http://127.0.0.1:8787/healthz',timeout=2)); raise SystemExit(0 if data.get('ok') else 1)"]
CMD ["/app/.venv/bin/python", "worker.py"]
