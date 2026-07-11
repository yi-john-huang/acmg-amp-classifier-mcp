# Python stdio MCP runtime. This image intentionally exposes no HTTP service.
FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    HOME=/data

WORKDIR /app

RUN groupadd --gid 10001 acmg && \
    useradd --uid 10001 --gid acmg --home-dir /data --create-home acmg

COPY pyproject.toml README.md LICENSE ./
COPY src ./src
RUN python -m pip install --no-cache-dir .

USER acmg
VOLUME ["/data"]

# MCP communicates exclusively through the container's standard streams.
ENTRYPOINT ["acmg-mcp"]
