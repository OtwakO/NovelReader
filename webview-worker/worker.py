"""Headless Patchright worker for NovelReader WebView requests."""

from __future__ import annotations

import asyncio
import json
import logging
import os
import time
from urllib.parse import urlparse

from patchright.async_api import async_playwright

PROTOCOL_VERSION = 1
MAX_REQUEST_BYTES = 1_048_576
DEFAULT_TIMEOUT_MS = 30_000


class BrowserWorker:
    def __init__(self, browser, max_pages: int, max_body_bytes: int):
        self.browser = browser
        self.pages = asyncio.Semaphore(max_pages)
        self.max_body_bytes = max_body_bytes

    async def execute(self, request: dict) -> dict:
        if request.get("version") != PROTOCOL_VERSION:
            return {"version": PROTOCOL_VERSION, "error": "unsupported protocol version"}
        target = request.get("url", "")
        if urlparse(target).scheme not in {"http", "https"}:
            return {"version": PROTOCOL_VERSION, "error": "only HTTP(S) URLs are supported"}
        if request.get("dnsIp"):
            return {"version": PROTOCOL_VERSION, "error": "dnsIp is unsupported by browser transport"}

        timeout = max(1, int(request.get("timeoutMs") or DEFAULT_TIMEOUT_MS))
        async with self.pages:
            context = None
            try:
                context = await self.browser.new_context(
                    extra_http_headers=request.get("headers") or {}
                )
                cookies = browser_cookies(request.get("cookies") or [], target)
                if cookies:
                    await context.add_cookies(cookies)
                page = await context.new_page()
                navigation_urls: list[str] = []
                page.on(
                    "request",
                    lambda event: navigation_urls.append(event.url)
                    if event.is_navigation_request()
                    else None,
                )
                method = (request.get("method") or "GET").upper()
                response = None
                if method == "GET":
                    response = await page.goto(
                        target, wait_until="domcontentloaded", timeout=timeout
                    )
                    await settle_page(page, timeout)
                else:
                    response = await context.request.fetch(
                        target,
                        method=method,
                        headers=request.get("headers") or {},
                        data=request.get("body") or None,
                        timeout=timeout,
                        fail_on_status_code=False,
                    )
                    body = await response.text()
                    ensure_body_size(body, self.max_body_bytes)
                    await page.set_content(
                        with_base_url(body, target),
                        wait_until="domcontentloaded",
                        timeout=timeout,
                    )

                delay_ms = max(0, int(request.get("delayMs") or 0))
                if delay_ms:
                    await asyncio.sleep(delay_ms / 1000)
                script = request.get("webJs") or ""
                if script:
                    await page.evaluate(
                        "async () => {" + script + "}", isolated_context=False
                    )
                    await settle_page(page, timeout)

                body = await page.content()
                ensure_body_size(body, self.max_body_bytes)
                headers = await response.all_headers() if response else {}
                final_url = page.url or target
                browser_state = await context.cookies()
                return {
                    "version": PROTOCOL_VERSION,
                    "statusCode": response.status if response else 200,
                    "headers": {key: [value] for key, value in headers.items()},
                    "body": body,
                    "finalUrl": final_url,
                    "redirectChain": navigation_urls[1:],
                    "cookies": protocol_cookies(browser_state),
                }
            except Exception as error:  # browser failures are returned to Go with context
                return {"version": PROTOCOL_VERSION, "error": f"browser execution failed: {error}"}
            finally:
                if context is not None:
                    await context.close()


async def settle_page(page, timeout_ms: int) -> None:
    try:
        await page.wait_for_load_state("networkidle", timeout=min(timeout_ms, 5_000))
    except Exception:
        # Pages with long-polling never become network-idle; DOM is still usable.
        pass


def browser_cookies(cookies: list[dict], target: str) -> list[dict]:
    result = []
    for cookie in cookies:
        item = {
            "name": cookie["name"],
            "value": cookie["value"],
            "path": cookie.get("path") or "/",
            "httpOnly": bool(cookie.get("httpOnly")),
            "secure": bool(cookie.get("secure")),
        }
        if cookie.get("domain"):
            item["domain"] = cookie["domain"]
        else:
            item["url"] = cookie.get("url") or target
        if cookie.get("expires"):
            item["expires"] = cookie["expires"]
        result.append(item)
    return result


def protocol_cookies(cookies: list[dict]) -> list[dict]:
    return [
        {
            "name": cookie["name"],
            "value": cookie["value"],
            "domain": cookie.get("domain", ""),
            "path": cookie.get("path", "/"),
            "expires": cookie.get("expires", -1),
            "httpOnly": cookie.get("httpOnly", False),
            "secure": cookie.get("secure", False),
        }
        for cookie in cookies
    ]


def with_base_url(body: str, target: str) -> str:
    base = f'<base href="{target}">'
    lower = body.lower()
    head = lower.find("<head")
    if head >= 0:
        end = body.find(">", head)
        if end >= 0:
            return body[: end + 1] + base + body[end + 1 :]
    return base + body


def ensure_body_size(body: str, max_bytes: int) -> None:
    if len(body.encode("utf-8")) > max_bytes:
        raise ValueError(f"response body exceeds {max_bytes} bytes")


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
    body = await reader.readexactly(length)
    return method, {"path": path, "headers": headers}, body


async def write_response(writer: asyncio.StreamWriter, status: int, body: dict) -> None:
    encoded = json.dumps(body, ensure_ascii=False).encode("utf-8")
    reason = "OK" if status == 200 else "Bad Request"
    header = (
        f"HTTP/1.1 {status} {reason}\r\n"
        f"Content-Type: application/json; charset=utf-8\r\n"
        f"Content-Length: {len(encoded)}\r\n"
        "Connection: close\r\n\r\n"
    ).encode("latin1")
    writer.write(header + encoded)
    await writer.drain()


async def serve_connection(reader: asyncio.StreamReader, writer: asyncio.StreamWriter, worker: BrowserWorker) -> None:
    try:
        method, request_meta, body = await read_request(reader)
        if method == "GET" and request_meta["path"] == "/healthz":
            await write_response(writer, 200, {"version": PROTOCOL_VERSION, "ok": True})
            return
        if method != "POST" or request_meta["path"] != "/execute":
            await write_response(writer, 404, {"error": "not found"})
            return
        request = json.loads(body)
        await write_response(writer, 200, await worker.execute(request))
    except Exception as error:
        await write_response(writer, 400, {"version": PROTOCOL_VERSION, "error": str(error)})
    finally:
        writer.close()
        await writer.wait_closed()


async def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))
    host = os.getenv("WEBVIEW_WORKER_HOST", "127.0.0.1")
    port = int(os.getenv("WEBVIEW_WORKER_PORT", "8787"))
    max_pages = max(1, int(os.getenv("WEBVIEW_MAX_PAGES", "4")))
    max_body = int(os.getenv("WEBVIEW_MAX_BODY_BYTES", str(10 * 1024 * 1024)))
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(headless=True)
        worker = BrowserWorker(browser, max_pages, max_body)
        server = await asyncio.start_server(
            lambda reader, writer: serve_connection(reader, writer, worker), host, port
        )
        logging.info("Patchright worker listening on %s:%d", host, port)
        async with server:
            await server.serve_forever()


if __name__ == "__main__":
    asyncio.run(main())
