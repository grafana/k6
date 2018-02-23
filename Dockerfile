FROM golang:1.9-alpine as builder
WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .
RUN apk --no-cache add --virtual .build-deps git make build-base && \
  go get . && CGO_ENABLED=0 go install -a -ldflags '-s -w' && \
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
