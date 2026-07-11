"""Resumable bundle download transport."""

import os
import urllib.request
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol
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

    def redirect_request(
        self,
        request: urllib.request.Request,
        fp: object,
        code: int,
        msg: str,
        headers: object,
        new_url: str,
    ) -> urllib.request.Request | None:
        _validate_download_url(new_url)
        return super().redirect_request(request, fp, code, msg, headers, new_url)


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
    ) -> None:
        if timeout_seconds <= 0 or chunk_size < 1 or max_download_bytes < 1:
            raise ValueError(
                "timeout_seconds, chunk_size, and max_download_bytes must be positive"
            )
        self.timeout_seconds = timeout_seconds
        self.chunk_size = chunk_size
        self.max_download_bytes = max_download_bytes

    def download(
        self,
        url: str,
        destination: Path,
        *,
        offset: int,
        progress: ProgressSink,
    ) -> None:
        """Append a valid partial response or safely restart a full response."""
        _validate_download_url(url)
        headers = {"Range": f"bytes={offset}-"} if offset else {}
        request = urllib.request.Request(url, headers=headers)
        destination.parent.mkdir(parents=True, exist_ok=True)
        opener = urllib.request.build_opener(_ValidatedRedirectHandler())
        with opener.open(request, timeout=self.timeout_seconds) as response:
            _validate_download_url(response.geturl())
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


def _validate_download_url(url: str) -> None:
    parsed = urlparse(url)
    if parsed.scheme == "https":
        return
    if parsed.scheme == "http" and parsed.hostname in {"127.0.0.1", "localhost", "::1"}:
        return
    raise ValueError("Bundle downloads require HTTPS except for loopback tests")
