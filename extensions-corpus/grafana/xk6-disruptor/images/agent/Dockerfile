FROM alpine:3.22@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412

ARG TARGETARCH

RUN apk update && apk add iproute2 iptables libc6-compat

WORKDIR /home/xk6-disruptor

COPY build/xk6-disruptor-agent-linux-${TARGETARCH} /usr/bin/xk6-disruptor-agent
