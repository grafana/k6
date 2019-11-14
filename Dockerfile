FROM golang:1.13-alpine as builder
WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .
RUN apk --no-cache add git
RUN CGO_ENABLED=0 go install -a -ldflags "-s -w -X github.com/loadimpact/k6/lib/consts.VersionDetails=$(date -u +"%FT%T%z")/$(git describe --always --long --dirty)"

FROM alpine:3.10
RUN apk add --no-cache ca-certificates
COPY --from=builder /go/bin/k6 /usr/bin/k6
ENTRYPOINT ["k6"]
