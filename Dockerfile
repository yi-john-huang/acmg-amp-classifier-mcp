# Python stdio MCP runtime. This image intentionally exposes no HTTP service.
FROM ghcr.io/astral-sh/uv:0.11.26 AS uv

FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    HOME=/data \
    PATH="/app/.venv/bin:$PATH"

WORKDIR /app

RUN groupadd --gid 10001 acmg && \
    useradd --uid 10001 --gid acmg --home-dir /data --create-home acmg

COPY --from=uv /uv /uvx /bin/

# Validate and install only the reviewed, production dependency set before source.
COPY pyproject.toml uv.lock README.md LICENSE ./
RUN uv sync --locked --no-dev --no-install-project

COPY src ./src
RUN uv sync --locked --no-dev --no-editable
USER acmg
VOLUME ["/data"]

# MCP communicates exclusively through the container's standard streams.
ENTRYPOINT ["acmg-mcp"]
