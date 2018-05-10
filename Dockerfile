FROM golang:1.9-alpine as builder

RUN apk --no-cache add --virtual .build-deps git make build-base

# Compile librdkafka from source
RUN echo '@edgecommunity http://nl.alpinelinux.org/alpine/edge/community' >> /etc/apk/repositories
RUN echo "@edge http://nl.alpinelinux.org/alpine/edge/main" >> /etc/apk/repositories
RUN apk --no-cache add bash openssl-dev librdkafka-dev@edgecommunity libressl2.7-libssl@edge

WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .

RUN go get . && CGO_ENABLED=1 go install -a -ldflags '-s -w' -tags static_all && \
  go get github.com/GeertJohan/go.rice && \
  cd $GOPATH/src/github.com/GeertJohan/go.rice/rice && \
  go get . && go install && \
  cd $GOPATH/src/github.com/loadimpact/k6 && \
  rice append --exec=$GOPATH/bin/k6 -i ./js/compiler -i ./js/lib

FROM alpine
WORKDIR /root/
COPY --from=builder /go/bin/k6 /root
COPY --from=builder /etc/ssl /etc/ssl
ENV PATH "$PATH:/root"
ENTRYPOINT ["./k6"]
