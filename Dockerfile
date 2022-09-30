FROM golang:1.19-bullseye as builder

RUN go install -trimpath go.k6.io/xk6/cmd/xk6@latest

RUN  xk6 build --output "/tmp/k6" --with github.com/grafana/xk6-browser

FROM debian:bullseye

ARG CHROMIUM_VERSION=106.0.5249.61-1~deb11u1

RUN apt-get update && \
    apt-get install -y chromium=${CHROMIUM_VERSION}

COPY --from=builder /tmp/k6 /usr/bin/k6

ENV XK6_HEADLESS=true

ENTRYPOINT ["k6"]