from __future__ import annotations

import ssl
import urllib.request
from email.message import Message
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from io import BytesIO
from threading import Thread
from urllib.response import addinfourl

import pytest


def test_redirect_handler_rejects_loopback_http_before_target_request() -> None:
    from acmg_classifier.infrastructure.bundles import transport

    class _InternalTargetHandler(BaseHTTPRequestHandler):
        requests = 0

        def do_GET(self) -> None:
            type(self).requests += 1
            self.send_response(200)
            self.end_headers()

        def log_message(self, format: str, *args: object) -> None:
            del format, args

    target_server = ThreadingHTTPServer(("127.0.0.1", 0), _InternalTargetHandler)
    target_thread = Thread(target=target_server.serve_forever)
    target_thread.start()
    target_url = f"http://127.0.0.1:{target_server.server_port}/internal"

    class _HttpsRedirector(urllib.request.HTTPSHandler):
        def https_open(self, request: urllib.request.Request) -> object:
            headers = Message()
            headers["Location"] = target_url
            response = addinfourl(BytesIO(), headers, request.full_url, 302)
            response.msg = "Found"
            return response

    opener = urllib.request.build_opener(
        _HttpsRedirector(), transport._ValidatedRedirectHandler()
    )
    try:
        with pytest.raises(ValueError, match="Bundle downloads require HTTPS"):
            opener.open("https://redirector.example.test/bundle")
    finally:
        target_server.shutdown()
        target_server.server_close()
        target_thread.join()

    assert _InternalTargetHandler.requests == 0

@pytest.mark.parametrize(
    "target_url",
    [
        "https://127.0.0.1/internal",
        "https://10.0.0.1/internal",
    ],
)
def test_redirect_handler_rejects_unsafe_https_ip_target(
    target_url: str,
) -> None:
    from acmg_classifier.infrastructure.bundles import transport

    handler = transport._ValidatedRedirectHandler()
    request = urllib.request.Request("https://bundles.example.test/current")

    with pytest.raises(ValueError, match="publicly routable"):
        handler.redirect_request(
            request,
            None,
            302,
            "Found",
            {},
            target_url,
        )


def test_redirect_handler_rejects_dns_rebound_target_before_following(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    from acmg_classifier.infrastructure.bundles import transport

    source_url = "https://bundles.example.test/current"
    rebound_url = "https://rebound.example.test/internal"
    target_requests = 0

    def _resolve(host: str, port: int, **_: object) -> list[tuple[object, ...]]:
        address = "127.0.0.1" if host == "rebound.example.test" else "93.184.216.34"
        return [(2, 1, 6, "", (address, port))]

    class _HttpsRedirector(urllib.request.HTTPSHandler):
        def https_open(self, request: urllib.request.Request) -> object:
            nonlocal target_requests
            if request.full_url == source_url:
                headers = Message()
                headers["Location"] = rebound_url
                response = addinfourl(BytesIO(), headers, source_url, 302)
                response.msg = "Found"
                return response
            target_requests += 1
            return addinfourl(BytesIO(), Message(), rebound_url, 200)

    monkeypatch.setattr(transport.socket, "getaddrinfo", _resolve)
    opener = urllib.request.build_opener(
        _HttpsRedirector(), transport._ValidatedRedirectHandler()
    )

    with pytest.raises(ValueError, match="publicly routable"):
        opener.open(source_url)

    assert target_requests == 0

@pytest.mark.parametrize(
    "url",
    [
        "http://127.0.0.1:8080/bundle",
        "http://localhost:8080/bundle",
        "http://[::1]:8080/bundle",
    ],
)
def test_download_url_allows_explicit_test_loopback_http_opt_in(url: str) -> None:
    from acmg_classifier.infrastructure.bundles import transport

    transport._validate_download_url(url, allow_loopback_http=True)


def test_pinned_https_connection_uses_validated_address_after_dns_changes(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    from acmg_classifier.infrastructure.bundles import transport

    resolution_attempts: list[str] = []
    connection_attempts: list[tuple[str, int]] = []

    def _resolve(host: str, port: int, **_: object) -> list[tuple[object, ...]]:
        resolution_attempts.append(host)
        address = "93.184.216.34" if len(resolution_attempts) == 1 else "127.0.0.1"
        return [(2, 1, 6, "", (address, port))]

    def _connect(
        address: tuple[str, int],
        timeout: float | None = None,
        source_address: tuple[str, int] | None = None,
    ) -> object:
        del timeout, source_address
        connection_attempts.append(address)
        return object()

    monkeypatch.setattr(transport.socket, "getaddrinfo", _resolve)
    monkeypatch.setattr(transport.socket, "create_connection", _connect)

    pinned_address = transport._validate_download_url(
        "https://bundles.example.test/current"
    )
    connection = transport._PinnedHTTPSConnection(
        "bundles.example.test", pinned_address=pinned_address
    )
    connection._create_connection(
        ("bundles.example.test", 443), None, None
    )

    assert resolution_attempts == ["bundles.example.test"]
    assert connection_attempts == [("93.184.216.34", 443)]

def test_pinned_https_handler_uses_its_ssl_context(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    from acmg_classifier.infrastructure.bundles import transport

    handler = transport._PinnedHTTPSHandler()
    expected = object()
    observed: dict[str, object] = {}

    def _do_open(
        connection_factory: object,
        request: urllib.request.Request,
        **kwargs: object,
    ) -> object:
        observed["connection_factory"] = connection_factory
        observed["request"] = request
        observed.update(kwargs)
        return expected

    monkeypatch.setattr(
        transport,
        "_validate_download_url",
        lambda *_args, **_kwargs: "93.184.216.34",
    )
    monkeypatch.setattr(handler, "do_open", _do_open)
    request = urllib.request.Request("https://bundles.example.test/current")

    assert handler.https_open(request) is expected
    assert observed["request"] is request
    assert isinstance(observed["context"], ssl.SSLContext)
    assert set(observed) == {"connection_factory", "context", "request"}


def test_download_opener_ignores_ambient_proxy_settings(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    from acmg_classifier.infrastructure.bundles import transport

    monkeypatch.setenv("HTTPS_PROXY", "http://127.0.0.1:8080")

    opener = transport._download_opener(allow_loopback_http=False)
    proxy_handlers = [
        handler
        for handler in opener.handlers
        if isinstance(handler, urllib.request.ProxyHandler)
    ]

    assert proxy_handlers == []



def test_redirect_handler_rejects_downgrade_before_following() -> None:
    from acmg_classifier.infrastructure.bundles import transport

    handler = transport._ValidatedRedirectHandler()
    request = urllib.request.Request("https://bundles.example.test/current")

    with pytest.raises(ValueError, match="Bundle downloads require HTTPS"):
        handler.redirect_request(
            request,
            None,
            302,
            "Found",
            {},
            "http://bundles.example.test/archive",
        )
