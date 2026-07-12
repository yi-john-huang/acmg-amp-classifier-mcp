"""Resumable bundle download transport."""

import http.client
import ipaddress
import os
import socket
import ssl
import urllib.request
from collections.abc import Callable
from dataclasses import dataclass
from functools import partial
from pathlib import Path
from typing import IO, Protocol
from urllib.parse import urlparse


class DownloadTransportError(RuntimeError):
    """Base class for bounded download protocol failures."""


class DownloadProtocolError(DownloadTransportError):
    """A server returned an invalid response for a resumed download."""


class DownloadTooLargeError(DownloadTransportError):
    """Compressed bundle bytes exceed the configured download bound."""


@dataclass(frozen=True, slots=True)
class ProgressEvent:
    """Presentation-neutral progress for a long-running bundle operation."""

    phase: str
    completed_bytes: int
    total_bytes: int | None


class _ValidatedRedirectHandler(urllib.request.HTTPRedirectHandler):
    """Reject unsafe redirect targets before urllib follows them."""

    def __init__(self, *, allow_loopback_http: bool = False) -> None:
        super().__init__()
        self._allow_loopback_http = allow_loopback_http

    def redirect_request(
        self,
        request: urllib.request.Request,
        fp: IO[bytes],
        code: int,
        msg: str,
        headers: http.client.HTTPMessage,
        new_url: str,
    ) -> urllib.request.Request | None:
        _validate_download_url(
            new_url, allow_loopback_http=self._allow_loopback_http
        )
        return super().redirect_request(request, fp, code, msg, headers, new_url)


def _connect_to_pinned_address(
    pinned_address: str,
    target: tuple[str, int],
    timeout: float | None,
    source_address: tuple[str, int] | None,
) -> socket.socket:
    return socket.create_connection(
        (pinned_address, target[1]), timeout, source_address
    )


class _PinnedHTTPSConnection(http.client.HTTPSConnection):
    """HTTPS connection whose TCP peer is a previously validated address."""

    def __init__(
        self,
        host: str,
        port: int | None = None,
        *,
        timeout: float | None = None,
        source_address: tuple[str, int] | None = None,
        context: ssl.SSLContext | None = None,
        blocksize: int = 8192,
        pinned_address: str,
    ) -> None:
        super().__init__(
            host,
            port=port,
            timeout=timeout,
            source_address=source_address,
            context=context,
            blocksize=blocksize,
        )
        self._create_connection = partial(
            _connect_to_pinned_address, pinned_address
        )


class _PinnedHTTPSHandler(urllib.request.HTTPSHandler):
    """Resolve and pin each direct HTTPS peer before connecting."""

    def __init__(
        self,
        *,
        allow_loopback_http: bool = False,
        ssl_context: ssl.SSLContext | None = None,
    ) -> None:
        self._allow_loopback_http = allow_loopback_http
        self._ssl_context = ssl_context or ssl.create_default_context()
        super().__init__(context=self._ssl_context)

    def https_open(
        self, request: urllib.request.Request
    ) -> http.client.HTTPResponse:
        pinned_address = _validate_download_url(
            request.full_url, allow_loopback_http=self._allow_loopback_http
        )
        if pinned_address is None:
            return super().https_open(request)
        connection_factory = partial(
            _PinnedHTTPSConnection, pinned_address=pinned_address
        )
        return self.do_open(
            connection_factory,
            request,
            context=self._ssl_context,
        )


def _download_opener(
    *, allow_loopback_http: bool
) -> urllib.request.OpenerDirector:
    """Create a direct-only opener so proxy DNS cannot bypass origin pinning."""
    return urllib.request.build_opener(
        urllib.request.ProxyHandler({}),
        _PinnedHTTPSHandler(allow_loopback_http=allow_loopback_http),
        _ValidatedRedirectHandler(allow_loopback_http=allow_loopback_http),
    )


type ProgressSink = Callable[[ProgressEvent], None]


class DownloadTransport(Protocol):
    """Port for resumable artifact downloads."""

    def download(
        self,
        url: str,
        destination: Path,
        *,
        offset: int,
        progress: ProgressSink,
    ) -> None: ...


class HttpDownloadTransport:
    """HTTPS downloader with Range resume and bounded streaming memory."""

    def __init__(
        self,
        *,
        timeout_seconds: float = 60,
        chunk_size: int = 64 * 1024,
        max_download_bytes: int = 300 * 1024 * 1024,
        allow_loopback_http: bool = False,
    ) -> None:
        if timeout_seconds <= 0 or chunk_size < 1 or max_download_bytes < 1:
            raise ValueError(
                "timeout_seconds, chunk_size, and max_download_bytes must be positive"
            )
        self.timeout_seconds = timeout_seconds
        self.chunk_size = chunk_size
        self.max_download_bytes = max_download_bytes
        self.allow_loopback_http = allow_loopback_http

    def download(
        self,
        url: str,
        destination: Path,
        *,
        offset: int,
        progress: ProgressSink,
    ) -> None:
        """Append a valid partial response or safely restart a full response."""
        _validate_download_url(url, allow_loopback_http=self.allow_loopback_http)
        headers = {"Range": f"bytes={offset}-"} if offset else {}
        request = urllib.request.Request(url, headers=headers)
        destination.parent.mkdir(parents=True, exist_ok=True)
        opener = _download_opener(
            allow_loopback_http=self.allow_loopback_http
        )
        with opener.open(request, timeout=self.timeout_seconds) as response:
            _validate_download_url(
                response.geturl(), allow_loopback_http=self.allow_loopback_http
            )
            status = response.status
            append = offset > 0 and status == 206
            if append:
                content_range = response.headers.get("Content-Range", "")
                if not content_range.startswith(f"bytes {offset}-"):
                    raise DownloadProtocolError(
                        f"Resume response does not start at byte {offset}"
                    )
            completed = offset if append else 0
            content_length = response.headers.get("Content-Length")
            total = completed + int(content_length) if content_length else None
            if total is not None and total > self.max_download_bytes:
                raise DownloadTooLargeError(
                    f"Bundle download is {total} bytes, above {self.max_download_bytes}"
                )
            mode = "ab" if append else "wb"
            with destination.open(mode) as output:
                while chunk := response.read(self.chunk_size):
                    output.write(chunk)
                    completed += len(chunk)
                    if completed > self.max_download_bytes:
                        raise DownloadTooLargeError(
                            f"Bundle download exceeds {self.max_download_bytes} bytes"
                        )
                    progress(ProgressEvent("download", completed, total))
                output.flush()
                os.fsync(output.fileno())



def _validate_public_host(hostname: str | None, port: int) -> str:
    """Resolve and return a public address, or reject the host."""
    if hostname is None:
        raise ValueError("Bundle download host must be publicly routable")
    try:
        addresses = socket.getaddrinfo(hostname, port, type=socket.SOCK_STREAM)
        resolved_addresses = [
            ipaddress.ip_address(address_info[4][0]) for address_info in addresses
        ]
    except (IndexError, OSError, ValueError) as error:
        raise ValueError("Bundle download host must be publicly routable") from error
    if not resolved_addresses or any(
        not address.is_global for address in resolved_addresses
    ):
        raise ValueError("Bundle download host must be publicly routable")
    return str(resolved_addresses[0])


def _validate_download_url(
    url: str, *, allow_loopback_http: bool = False
) -> str | None:
    parsed = urlparse(url)
    if (
        allow_loopback_http
        and parsed.scheme == "http"
        and parsed.hostname in {"127.0.0.1", "localhost", "::1"}
    ):
        return None
    if parsed.scheme != "https":
        raise ValueError("Bundle downloads require HTTPS")
    try:
        port = parsed.port or 443
    except ValueError as error:
        raise ValueError("Bundle download host must be publicly routable") from error
    return _validate_public_host(parsed.hostname, port)
