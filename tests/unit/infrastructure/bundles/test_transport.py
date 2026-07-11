from __future__ import annotations

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
