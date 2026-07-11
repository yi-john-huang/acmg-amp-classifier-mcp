from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any

import pytest
from tests.integration.data_builder.test_core_bundle_builder import _recipe


class _FakeResponse:
    def __init__(self, content: bytes) -> None:
        self._content = content
        self._read = False

    def __enter__(self) -> _FakeResponse:
        return self

    def __exit__(self, *args: Any) -> None:
        return None

    def read(self, _size: int = -1) -> bytes:
        if self._read:
            return b""
        self._read = True
        return self._content


def _loaded_recipe(tmp_path: Path):
    from data_builder.builder import load_recipe

    return load_recipe(_recipe(tmp_path))


def test_existing_checksum_valid_cache_skips_download(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from data_builder import sources

    recipe = _loaded_recipe(tmp_path)
    cache = tmp_path / "cache"
    cache.mkdir()
    for source in recipe.sources:
        content = (tmp_path / "inputs" / source.filename).read_bytes()
        (cache / source.filename).write_bytes(content)

    def unexpected_download(*_args: Any, **_kwargs: Any) -> Any:
        raise AssertionError("valid source cache must not open a network request")

    monkeypatch.setattr(sources, "_open_https_source", unexpected_download)

    assert sources.acquire_sources(recipe, cache) == tuple(
        cache / source.filename for source in recipe.sources
    )


def test_checksum_mismatch_removes_partial_and_fails_closed(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from data_builder import sources

    recipe = _loaded_recipe(tmp_path)
    cache = tmp_path / "cache"
    cache.mkdir()

    monkeypatch.setattr(
        sources,
        "_open_https_source",
        lambda *_args, **_kwargs: _FakeResponse(b"not-the-locked-source"),
    )

    with pytest.raises(sources.SourceAcquisitionError, match="checksum mismatch"):
        sources.acquire_sources(recipe, cache)

    assert not tuple(cache.iterdir())


def test_oversized_download_removes_partial(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from data_builder import sources

    recipe = _loaded_recipe(tmp_path)
    cache = tmp_path / "cache"
    monkeypatch.setattr(sources, "MAX_SOURCE_BYTES", 3)
    monkeypatch.setattr(
        sources,
        "_open_https_source",
        lambda *_args, **_kwargs: _FakeResponse(b"1234"),
    )

    with pytest.raises(sources.SourceAcquisitionError, match="exceeds 3 bytes"):
        sources.acquire_sources(recipe, cache)

    assert not tuple(cache.iterdir())


def test_download_failure_removes_partial_file(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from data_builder import sources

    recipe = _loaded_recipe(tmp_path)
    cache = tmp_path / "cache"

    def failed_download(*_args: Any, **_kwargs: Any) -> Any:
        raise OSError("network unavailable")

    monkeypatch.setattr(sources, "_open_https_source", failed_download)

    with pytest.raises(OSError, match="network unavailable"):
        sources.acquire_sources(recipe, cache)

    assert not tuple(cache.iterdir())


def test_staging_does_not_collide_with_another_locked_filename(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from data_builder import sources
    from data_builder.builder import load_recipe

    recipe_path = _recipe(tmp_path)
    recipe_data = json.loads(recipe_path.read_text())
    contents = {
        "https://example.test/mane": b"mane bytes",
        "https://example.test/clingen": b"clingen bytes",
    }
    for source, filename, url in zip(
        recipe_data["sources"],
        ("foo.partial", "foo"),
        contents,
        strict=True,
    ):
        source["filename"] = filename
        source["url"] = url
        source["sha256"] = hashlib.sha256(contents[url]).hexdigest()
    recipe_path.write_text(json.dumps(recipe_data))
    recipe = load_recipe(recipe_path)
    cache = tmp_path / "cache"

    def downloaded(request: Any, **_kwargs: Any) -> _FakeResponse:
        return _FakeResponse(contents[request.full_url])

    monkeypatch.setattr(sources, "_open_https_source", downloaded)

    assert sources.acquire_sources(recipe, cache) == (
        cache / "foo.partial",
        cache / "foo",
    )
    assert (cache / "foo.partial").read_bytes() == contents["https://example.test/mane"]
    assert (cache / "foo").read_bytes() == contents["https://example.test/clingen"]


def test_https_redirect_handler_rejects_http_destination() -> None:
    from data_builder import sources

    handler = sources._HTTPSOnlyRedirectHandler()
    request = sources.urllib.request.Request("https://example.test/source")

    with pytest.raises(sources.SourceAcquisitionError, match="non-HTTPS redirect"):
        handler.redirect_request(
            request, None, 302, "Found", {}, "http://example.test/source"
        )
