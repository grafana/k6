# syntax=docker/dockerfile:1
# Build
FROM --platform=$BUILDPLATFORM golang:1.20-alpine AS builder

ARG TARGETOS TARGETARCH

ENV GOOS=$TARGETOS \
  GOARCH=$TARGETARCH \
  CGO_ENABLED=0

WORKDIR $GOPATH/src/go.k6.io/k6
COPY . .
RUN apk --no-cache add git=~2
RUN go build -a -trimpath -ldflags "-s -w -X go.k6.io/k6/lib/consts.VersionDetails=$(date -u +"%FT%T%z")/$(git describe --tags --always --long --dirty)" -o /usr/bin/k6 .

# Runtime stage
FROM alpine:3.17
RUN apk add --no-cache ca-certificates=~20220614 && \
    adduser -D -u 12345 -g 12345 k6
COPY --from=builder /usr/bin/k6 /usr/bin/k6

USER 12345
WORKDIR /home/k6
ENTRYPOINT ["k6"]
