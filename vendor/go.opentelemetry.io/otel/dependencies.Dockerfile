# This is a renovate-friendly source of Docker images.
FROM python:3.13.5-slim-bullseye@sha256:5b9fc0d8ef79cfb5f300e61cb516e0c668067bbf77646762c38c94107e230dbc AS python
FROM otel/weaver:v0.18.0@sha256:5425ade81dc22ddd840902b0638b4b6a9186fb654c5b50c1d1ccd31299437390 AS weaver
FROM avtodev/markdown-lint:v1@sha256:6aeedc2f49138ce7a1cd0adffc1b1c0321b841dc2102408967d9301c031949ee AS markdown
