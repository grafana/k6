# This is a renovate-friendly source of Docker images.
FROM python:3.13.5-slim-bullseye@sha256:846d391e752027be4a490ac42f8bba40b54b8271fd57d18d5304698a02a4efb0 AS python
FROM otel/weaver:v0.15.2@sha256:b13acea09f721774daba36344861f689ac4bb8d6ecd94c4600b4d590c8fb34b9 AS weaver
FROM avtodev/markdown-lint:v1@sha256:6aeedc2f49138ce7a1cd0adffc1b1c0321b841dc2102408967d9301c031949ee AS markdown
