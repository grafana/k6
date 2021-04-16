FROM debian:buster-20210311

LABEL maintainer="k6 Developers <developers@k6.io>"

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update -y && \
    apt-get install -y apt-utils createrepo curl git gnupg2 python3-pip unzip

ARG S3CMD_VERSION
RUN pip3 install "s3cmd${S3CMD_VERSION:+==$S3CMD_VERSION}"

COPY ./awscli-key.gpg .
ARG AWSCLI_VERSION
# Download awscli, check GPG signature and install.
RUN export GNUPGHOME="$(mktemp -d)" && \
    gpg2 --import ./awscli-key.gpg && \
    fpr="$(gpg2 --with-colons --fingerprint aws-cli | grep '^fpr' | cut -d: -f10)" && \
    gpg2 --export-ownertrust && echo "${fpr}:6:" | gpg2 --import-ownertrust && \
    curl -fsSL --remote-name-all \
      "https://awscli.amazonaws.com/awscli-exe-linux-x86_64${AWSCLI_VERSION:+-$AWSCLI_VERSION}.zip"{,.sig} && \
    gpg2 --verify awscli*.sig awscli*.zip && \
    unzip -q awscli*.zip && \
    ./aws/install && \
    rm -rf aws* "$GNUPGHOME"

RUN addgroup --gid 1000 k6 && \
    useradd --create-home --shell /bin/bash --no-log-init \
      --uid 1000 --gid 1000 k6

COPY bin/ /usr/local/bin/

USER k6
WORKDIR /home/k6

COPY --chown=k6:k6 ./k6-rpm-repo.spec rpmbuild/SPECS/
COPY --chown=k6:k6 ./k6-rpm.repo rpmbuild/SOURCES/k6-io.repo

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
