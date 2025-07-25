FROM debian:buster-20240612

LABEL maintainer="k6 Developers <developers@k6.io>"

ENV DEBIAN_FRONTEND=noninteractive

RUN sed -i 's|http://deb.debian.org/debian|http://archive.debian.org/debian|g' /etc/apt/sources.list && \
    sed -i 's|http://security.debian.org/debian-security|http://archive.debian.org/debian-security|g' /etc/apt/sources.list && \
    echo 'Acquire::Check-Valid-Until "false";' > /etc/apt/apt.conf.d/99no-check-valid-until && \
    apt-get update -y && \
    apt-get install -y apt-utils createrepo curl git gnupg2 python3-pip unzip

RUN pip3 install "s3cmd==2.4.0"

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
