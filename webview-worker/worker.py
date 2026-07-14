"""Headless Patchright HTTP server for NovelReader WebView requests."""

from __future__ import annotations

import asyncio
import json
import logging
import os
import signal

from patchright.async_api import async_playwright

from browser import BrowserWorker, PROTOCOL_VERSION

MAX_REQUEST_BYTES = 1_048_576


async def read_request(reader: asyncio.StreamReader) -> tuple[str, dict, bytes]:
    request_line = await reader.readline()
    if not request_line:
        raise ValueError("empty request")
    method, path, _ = request_line.decode("latin1").strip().split(" ", 2)
    headers: dict[str, str] = {}
    while True:
        line = await reader.readline()
        if line in (b"\r\n", b"\n", b""):
            break
        key, value = line.decode("latin1").split(":", 1)
        headers[key.lower().strip()] = value.strip()
    length = int(headers.get("content-length", "0"))
    if length > MAX_REQUEST_BYTES:
        raise ValueError("request body too large")
    return method, {"path": path, "headers": headers}, await reader.readexactly(length)


async def write_response(writer: asyncio.StreamWriter, status: int, body: dict) -> None:
    encoded = json.dumps(body, ensure_ascii=False).encode("utf-8")
    reason = {200: "OK", 400: "Bad Request", 404: "Not Found", 503: "Busy"}.get(status, "Error")
    header = (
        f"HTTP/1.1 {status} {reason}\r\n"
        "Content-Type: application/json; charset=utf-8\r\n"
        f"Content-Length: {len(encoded)}\r\n"
        "Connection: close\r\n\r\n"
    ).encode("latin1")
    writer.write(header + encoded)
    await writer.drain()


async def serve_connection(
    reader: asyncio.StreamReader, writer: asyncio.StreamWriter, worker: BrowserWorker
) -> None:
    try:
        method, request_meta, body = await read_request(reader)
        if method == "GET" and request_meta["path"] == "/healthz":
            await write_response(writer, 200, await worker.health())
            return
        if method != "POST" or request_meta["path"] != "/execute":
            await write_response(writer, 404, {"error": "not found"})
            return
        result = await worker.submit(json.loads(body))
        await write_response(writer, 503 if result.get("error") == "browser worker is busy" else 200, result)
    except Exception as error:
        await write_response(writer, 400, {"version": PROTOCOL_VERSION, "error": str(error)})
    finally:
        writer.close()
        await writer.wait_closed()


def capacity_settings() -> tuple[int, int, int, int]:
    """Load bounded worker capacity, using small-container defaults."""
    return (
        max(1, int(os.getenv("WEBVIEW_MAX_PAGES", "2"))),
        max(1, int(os.getenv("WEBVIEW_MAX_PENDING", "8"))),
        int(os.getenv("WEBVIEW_MAX_BODY_BYTES", str(10 * 1024 * 1024))),
        max(1, int(os.getenv("WEBVIEW_MAX_CONTEXTS_PER_BROWSER", "100"))),
    )


async def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))
    host = os.getenv("WEBVIEW_WORKER_HOST", "127.0.0.1")
    port = int(os.getenv("WEBVIEW_WORKER_PORT", "8787"))
    max_pages, max_pending, max_body, max_contexts = capacity_settings()
    stop = asyncio.Event()

    async with async_playwright() as playwright:
        worker = BrowserWorker(playwright, max_pages, max_pending, max_body, max_contexts)
        await worker.start()
        server = await asyncio.start_server(
            lambda reader, writer: serve_connection(reader, writer, worker), host, port
        )
        loop = asyncio.get_running_loop()
        for signum in (signal.SIGINT, signal.SIGTERM):
            try:
                loop.add_signal_handler(signum, stop.set)
            except NotImplementedError:
                pass
        logging.info("Patchright worker listening on %s:%d", host, port)
        async with server:
            await stop.wait()
        await worker.close()


if __name__ == "__main__":
    asyncio.run(main())
