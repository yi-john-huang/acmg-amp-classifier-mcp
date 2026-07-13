from __future__ import annotations

import socket
import ssl
from datetime import UTC, datetime, timedelta
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID


def _write_test_certificate(directory: Path, hostname: str) -> tuple[Path, Path]:
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, hostname)])
    now = datetime.now(UTC)
    certificate = (
        x509.CertificateBuilder()
        .subject_name(name)
        .issuer_name(name)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - timedelta(minutes=1))
        .not_valid_after(now + timedelta(days=1))
        .add_extension(
            x509.SubjectAlternativeName([x509.DNSName(hostname)]), critical=False
        )
        .sign(key, hashes.SHA256())
    )
    certificate_path = directory / "server.crt"
    key_path = directory / "server.key"
    certificate_path.write_bytes(certificate.public_bytes(serialization.Encoding.PEM))
    key_path.write_bytes(
        key.private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.TraditionalOpenSSL,
            serialization.NoEncryption(),
        )
    )
    return certificate_path, key_path


def _bundle_handler(payload: bytes) -> type[BaseHTTPRequestHandler]:
    class BundleHandler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            self.send_response(200)
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, format: str, *args: object) -> None:
            del format, args

    return BundleHandler


def test_download_uses_pinned_https_connection_end_to_end(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    from acmg_classifier.infrastructure.bundles import transport

    hostname = "bundles.example.test"
    pinned_address = "93.184.216.34"
    payload = b"bundle-bytes"
    certificate_path, key_path = _write_test_certificate(tmp_path, hostname)

    server_context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    server_context.load_cert_chain(certificate_path, key_path)
    server = ThreadingHTTPServer(("127.0.0.1", 0), _bundle_handler(payload))
    server.socket = server_context.wrap_socket(server.socket, server_side=True)
    server_thread = Thread(target=server.serve_forever)
    server_thread.start()

    original_create_context = ssl.create_default_context
    original_create_connection = transport.socket.create_connection
    original_getaddrinfo = transport.socket.getaddrinfo
    connection_attempts: list[tuple[str, int]] = []

    def resolve(
        host: str, port: int, *args: object, **kwargs: object
    ) -> list[tuple[object, ...]]:
        if host == hostname:
            return [
                (
                    transport.socket.AF_INET,
                    transport.socket.SOCK_STREAM,
                    transport.socket.IPPROTO_TCP,
                    "",
                    (pinned_address, port),
                )
            ]
        return original_getaddrinfo(host, port, *args, **kwargs)

    def connect(
        address: tuple[str, int],
        timeout: float | None = None,
        source_address: tuple[str, int] | None = None,
    ) -> socket.socket:
        connection_attempts.append(address)
        if address[0] == pinned_address:
            address = ("127.0.0.1", address[1])
        return original_create_connection(
            address, timeout=timeout, source_address=source_address
        )

    client_context = original_create_context(cafile=str(certificate_path))
    monkeypatch.setattr(transport.ssl, "create_default_context", lambda: client_context)
    monkeypatch.setattr(transport.socket, "getaddrinfo", resolve)
    monkeypatch.setattr(transport.socket, "create_connection", connect)

    progress: list[transport.ProgressEvent] = []
    destination = tmp_path / "bundle.bin"
    try:
        downloader = transport.HttpDownloadTransport(
            timeout_seconds=5,
            chunk_size=3,
            max_download_bytes=len(payload),
        )
        downloader.download(
            f"https://{hostname}:{server.server_port}/bundle",
            destination,
            offset=0,
            progress=progress.append,
        )
    finally:
        server.shutdown()
        server.server_close()
        server_thread.join(timeout=5)

    assert not server_thread.is_alive()
    assert destination.read_bytes() == payload
    assert connection_attempts == [(pinned_address, server.server_port)]
    assert progress
    assert progress[-1].completed_bytes == len(payload)
    assert progress[-1].total_bytes == len(payload)
