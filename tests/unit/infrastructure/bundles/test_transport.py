from __future__ import annotations

import urllib.request

import pytest


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
