FROM debian:buster-20210311

LABEL maintainer="k6 Developers <developers@k6.io>"

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update -y && \
    apt-get install -y apt-utils createrepo curl git gnupg2 python3-pip

RUN pip3 install s3cmd

RUN addgroup --gid 1000 k6 && \
    useradd --create-home --shell /bin/bash --no-log-init \
      --uid 1000 --gid 1000 k6

COPY bin/ /usr/local/bin/

USER k6
WORKDIR /home/k6

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
