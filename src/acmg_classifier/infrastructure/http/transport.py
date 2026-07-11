"""HTTPX-backed transport used by evidence sources in production."""

from __future__ import annotations

from collections.abc import AsyncIterator

import httpx

from acmg_classifier.infrastructure.http.policy import HttpRequest, HttpResponse


class HTTPXTransport:
    """Certificate-verifying streaming HTTPX transport with redirects disabled."""

    def __init__(self, client: httpx.AsyncClient | None = None) -> None:
        self._client = client or httpx.AsyncClient(verify=True, follow_redirects=False)
        self._owns_client = client is None

    async def request(self, request: HttpRequest) -> HttpResponse:
        outbound = self._client.build_request(
            request.method,
            request.url,
            headers=request.headers,
            content=request.body,
        )
        response = await self._client.send(
            outbound, stream=True, follow_redirects=False
        )

        async def body() -> AsyncIterator[bytes]:
            async for chunk in response.aiter_bytes():
                yield chunk

        return HttpResponse(
            status_code=response.status_code,
            headers=dict(response.headers),
            body=body(),
            close=response.aclose,
        )

    async def aclose(self) -> None:
        if self._owns_client:
            await self._client.aclose()
