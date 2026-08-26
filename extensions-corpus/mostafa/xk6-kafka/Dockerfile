FROM ubuntu:24.04

ARG VERSION_TAG
ARG TARGETOS
ARG TARGETARCH

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates librdkafka1 openssl && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -g 12345 k6 && \
    useradd -u 12345 -g 12345 -m -s /bin/sh k6
COPY --chmod=0755 ./dist/xk6-kafka_${VERSION_TAG}_${TARGETOS}_${TARGETARCH} /usr/bin/k6

USER 12345
WORKDIR /home/k6
ENTRYPOINT ["k6"]
