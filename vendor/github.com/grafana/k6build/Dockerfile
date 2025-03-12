ARG GO_VERSION=1.22.4
ARG VARIANT=bookworm
FROM golang:${GO_VERSION}-${VARIANT} as builder

WORKDIR /build

COPY . .

ARG GOFLAGS="-ldflags=-w -ldflags=-s"
RUN CGO_ENABLED=0 go build -o k6build -trimpath ./cmd/k6build/main.go

# k6build server requires golang toolchain
FROM golang:${GO_VERSION}-${VARIANT}

RUN addgroup --gid 1000 k6build && \
    adduser --uid 1000 --ingroup k6build \
    --home /home/k6build --shell /bin/sh \
    --disabled-password --gecos "" k6build

COPY --from=builder /build/k6build /usr/local/bin/

WORKDIR /home/k6build

USER k6build

ENTRYPOINT ["/usr/local/bin/k6build"]