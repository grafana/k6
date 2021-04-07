FROM debian:buster-20210311

LABEL maintainer="k6 Developers <developers@k6.io>"

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update -y && \
    apt-get install -y apt-utils createrepo curl git gnupg2 python3 unzip

ARG AWSCLI_VERSION=2.1.35

RUN curl -fSsL -o "awscliv2.zip" \
      "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-${AWSCLI_VERSION}.zip" && \
    unzip -q awscliv2.zip && \
    ./aws/install && \
    rm -rf aws*

RUN addgroup --gid 1000 k6 && \
    useradd --create-home --shell /bin/bash --no-log-init \
      --uid 1000 --gid 1000 k6

COPY bin/ /usr/local/bin/

USER k6
WORKDIR /home/k6

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
