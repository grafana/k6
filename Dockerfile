FROM golang:1.20-alpine3.17 as builder
WORKDIR $GOPATH/src/go.k6.io/k6
COPY . .
RUN apk --no-cache add git=~2
RUN CGO_ENABLED=0 go install -a -trimpath -ldflags "-s -w -X go.k6.io/k6/lib/consts.VersionDetails=$(date -u +"%FT%T%z")/$(git describe --tags --always --long --dirty)"

# Runtime stage
FROM alpine:3.17 as release

# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 12345 -g 12345 k6
COPY --from=builder /go/bin/k6 /usr/bin/k6

USER k6
WORKDIR /home/k6

ENTRYPOINT ["k6"]

# Legacy loadimpact/k6 image
FROM release

COPY entrypoint-legacy.sh /usr/bin/

ENTRYPOINT ["/usr/bin/entrypoint-legacy.sh"]
