FROM debian:trixie-20260406@sha256:3352c2e13876c8a5c5873ef20870e1939e73cb9a3c1aeba5e3e72172a85ce9ed

LABEL maintainer="k6 Developers <developers@k6.io>"

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update -y && \
    apt-get install -y apt-utils createrepo-c curl git gnupg2 s3cmd unzip rpm

# Download awscli, check GPG signature and install.
COPY ./awscli-key.gpg .
RUN export GNUPGHOME="$(mktemp -d)" && \
    gpg2 --import ./awscli-key.gpg && \
    fpr="$(gpg2 --with-colons --fingerprint aws-cli | grep '^fpr' | cut -d: -f10)" && \
    gpg2 --export-ownertrust && echo "${fpr}:6:" | gpg2 --import-ownertrust && \
    curl -fsSL --remote-name-all \
      "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-2.26.5.zip"{,.sig} && \
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
