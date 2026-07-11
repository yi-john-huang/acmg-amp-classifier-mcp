"""Checksum-pinned acquisition of official bundle inputs."""

import hashlib
import os
import tempfile
import urllib.request
from pathlib import Path
from urllib.parse import urlsplit

from data_builder.models import BuildRecipe

MAX_SOURCE_BYTES = 100 * 1024 * 1024


class SourceAcquisitionError(RuntimeError):
    """An upstream file could not be acquired exactly as locked."""


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


class _HTTPSOnlyRedirectHandler(urllib.request.HTTPRedirectHandler):
    """Reject redirects that would move locked source bytes off HTTPS."""

    def redirect_request(
        self,
        request: urllib.request.Request,
        file_pointer: object,
        code: int,
        message: str,
        headers: object,
        new_url: str,
    ) -> urllib.request.Request | None:
        if urlsplit(new_url).scheme != "https":
            raise SourceAcquisitionError(
                f"non-HTTPS redirect rejected for {request.full_url}"
            )
        return super().redirect_request(
            request, file_pointer, code, message, headers, new_url
        )


def _open_https_source(request: urllib.request.Request):
    opener = urllib.request.build_opener(_HTTPSOnlyRedirectHandler)
    return opener.open(request, timeout=60)


def acquire_sources(recipe: BuildRecipe, input_directory: Path) -> tuple[Path, ...]:
    """Download absent inputs and verify every byte against the source lock."""
    input_directory.mkdir(parents=True, exist_ok=True)
    acquired: list[Path] = []
    for source in recipe.sources:
        target = input_directory / source.filename
        if target.exists() and file_sha256(target) == source.sha256:
            acquired.append(target)
            continue
        request = urllib.request.Request(
            str(source.url), headers={"User-Agent": "acmg-classifier-data-builder/1.0"}
        )
        digest = hashlib.sha256()
        total = 0
        temporary: Path | None = None
        try:
            with tempfile.NamedTemporaryFile(
                mode="wb",
                dir=input_directory,
                prefix=f".{source.identifier}.",
                suffix=".partial",
                delete=False,
            ) as output:
                temporary = Path(output.name)
                with _open_https_source(request) as response:
                    while block := response.read(1024 * 1024):
                        total += len(block)
                        if total > MAX_SOURCE_BYTES:
                            raise SourceAcquisitionError(
                                f"source exceeds {MAX_SOURCE_BYTES} bytes: "
                                f"{source.identifier}"
                            )
                        digest.update(block)
                        output.write(block)
            if digest.hexdigest() != source.sha256:
                raise SourceAcquisitionError(
                    f"source checksum mismatch for {source.identifier}"
                )
            os.replace(temporary, target)
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)
        acquired.append(target)
    return tuple(acquired)
