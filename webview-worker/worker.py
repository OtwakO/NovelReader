"""Patchright HTTP server for NovelReader WebView requests."""

from __future__ import annotations

import asyncio
import json
import logging
import os
import signal
from typing import cast

from patchright.async_api import async_playwright

from browser import BrowserWorker, PROTOCOL_VERSION
from runtime import DEFAULT_TIMEOUT_MS, WorkerBusyError, VirtualDisplay, browser_mode, runtime_metadata

MAX_REQUEST_BYTES = 1_048_576
SHUTDOWN_GRACE_SECONDS = 10


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
    body.setdefault("version", PROTOCOL_VERSION)
    encoded = json.dumps(body, ensure_ascii=False).encode("utf-8")
    reason = {200: "OK", 400: "Bad Request", 404: "Not Found", 503: "Busy", 504: "Gateway Timeout"}.get(status, "Error")
    header = (
        f"HTTP/1.1 {status} {reason}\r\n"
        "Content-Type: application/json; charset=utf-8\r\n"
        f"Content-Length: {len(encoded)}\r\n"
        "Connection: close\r\n\r\n"
    ).encode("latin1")
    writer.write(header + encoded)
    try:
        await asyncio.wait_for(writer.drain(), DEFAULT_TIMEOUT_MS / 1000)
    except TimeoutError as error:
        raise ConnectionError("worker response write timed out") from error


async def serve_connection(
    reader: asyncio.StreamReader, writer: asyncio.StreamWriter, worker: BrowserWorker
) -> None:
    try:
        if worker.closing:
            await write_response(writer, 503, {"error": "browser worker is shutting down"})
            return
        async with asyncio.timeout(DEFAULT_TIMEOUT_MS / 1000):
            method, request_meta, body = await read_request(reader)
        if method == "GET" and request_meta["path"] == "/healthz":
            await write_response(writer, 200, await worker.health())
            return
        path = request_meta["path"]
        payload = json.loads(body) if body else {}
        if method == "POST" and path == "/execute":
            result = await worker.submit(payload)
        elif method == "POST" and path == "/sessions":
            result = await worker.open_interactive(payload)
        elif path.startswith("/sessions/"):
            parts = path.split("/")
            session_id = parts[2] if len(parts) > 2 else ""
            if method == "GET" and len(parts) == 4 and parts[3] == "frame":
                result = await worker.interactive.frame(session_id)
            elif method == "POST" and len(parts) == 4 and parts[3] == "input":
                result = await worker.interactive.input(session_id, payload)
            elif method == "DELETE" and len(parts) == 3:
                result = await worker.interactive.close(session_id, save=bool(payload.get("save")), return_html=bool(payload.get("returnHtml")))
            else:
                await write_response(writer, 404, {"error": "not found"})
                return
        else:
            await write_response(writer, 404, {"error": "not found"})
            return
        await write_response(writer, 503 if result.get("error") == "browser worker is busy" else 200, result)
    except ConnectionError:
        raise  # Do not attempt a second HTTP response on a failed writer.
    except TimeoutError:
        await write_response(writer, 504, {"error": "browser request timed out"})
    except WorkerBusyError as error:
        await write_response(writer, 503, {"error": str(error)})
    except Exception as error:
        await write_response(writer, 400, {"version": PROTOCOL_VERSION, "error": str(error)})
    finally:
        writer.close()
        try:
            await asyncio.wait_for(writer.wait_closed(), 1)
        except (ConnectionError, TimeoutError):
            pass


def capacity_settings() -> tuple[int, int, int, int, int, int]:
    """Load bounded worker capacity, using small-container defaults."""
    return (
        max(1, int(os.getenv("WEBVIEW_MAX_PAGES", "2"))),
        max(1, int(os.getenv("WEBVIEW_MAX_PENDING", "8"))),
        int(os.getenv("WEBVIEW_MAX_BODY_BYTES", str(10 * 1024 * 1024))),
        max(1, int(os.getenv("WEBVIEW_MAX_CONTEXTS_PER_BROWSER", "100"))),
        max(10, int(os.getenv("WEBVIEW_INTERACTIVE_IDLE_SECONDS", "120"))),
        max(30, int(os.getenv("WEBVIEW_INTERACTIVE_ABSOLUTE_SECONDS", "600"))),
    )


async def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))
    host = os.getenv("WEBVIEW_WORKER_HOST", "127.0.0.1")
    port = int(os.getenv("WEBVIEW_WORKER_PORT", "8787"))
    mode = browser_mode()
    runtime_info = runtime_metadata()
    max_pages, max_pending, max_body, max_contexts, interactive_idle, interactive_absolute = capacity_settings()
    display = VirtualDisplay(mode)
    playwright = None
    worker = None
    server = None
    connections: set[asyncio.Task] = set()
    loop = asyncio.get_running_loop()

    def force_exit() -> None:
        logging.critical("browser worker shutdown exceeded its deadline; terminating worker")
        # Container PID 1 exit / the native service supervisor owns descendant
        # termination. Never respawn Python or Chrome inside this failed runtime.
        os._exit(1)

    async def connection(reader, writer) -> None:
        # asyncio schedules this callback as a task, only after worker startup.
        task = cast(asyncio.Task, asyncio.current_task())
        connections.add(task)
        try:
            await serve_connection(reader, writer, cast(BrowserWorker, worker))
        except ConnectionError:
            pass  # The client disconnected; serve_connection still closes its writer.
        except Exception:
            logging.exception("worker connection failed")
        finally:
            connections.discard(task)

    try:
        await display.start()
        playwright = await async_playwright().start()
        worker = BrowserWorker(
            playwright, max_pages, max_pending, max_body, max_contexts,
            interactive_idle, interactive_absolute, mode, runtime_info,
        )
        await worker.start()
        server = await asyncio.start_server(connection, host, port)
        for signum in (signal.SIGINT, signal.SIGTERM):
            try:
                loop.add_signal_handler(signum, worker.request_shutdown)
            except NotImplementedError:
                pass
        logging.info("Patchright %s worker listening on %s:%d", mode, host, port)
        await worker.shutdown_requested.wait()
    finally:
        # Arm before any await, including server/request and driver teardown.
        # Cooperative cancellation alone cannot guarantee a bounded shutdown.
        watchdog = loop.call_later(SHUTDOWN_GRACE_SECONDS, force_exit)
        cleanup_complete = False
        try:
            if server is not None:
                server.close()
            for task in list(connections):
                task.cancel()
            await asyncio.gather(*list(connections), return_exceptions=True)
            try:
                if worker is not None:
                    await worker.close()
            finally:
                try:
                    if playwright is not None:
                        await playwright.stop()
                finally:
                    await display.close()
            if server is not None:
                await server.wait_closed()
            cleanup_complete = True
        finally:
            # Keep the deadline armed on failure while asyncio.run drains any
            # remaining tasks; they may also refuse cooperative cancellation.
            if cleanup_complete and (worker is None or worker.failure is None):
                watchdog.cancel()
    if worker is not None and worker.failure is not None:
        raise RuntimeError(worker.failure)


if __name__ == "__main__":
    asyncio.run(main())
