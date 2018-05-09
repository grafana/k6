FROM golang:1.9-alpine as builder

RUN apk --no-cache add --virtual .build-deps git make build-base

# Compile librdkafka from source
RUN apk --no-cache add --virtual bash
WORKDIR /root
RUN git clone https://github.com/edenhill/librdkafka.git
WORKDIR /root/librdkafka
RUN git checkout v0.11.4
RUN /root/librdkafka/configure
RUN make
RUN make install
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .

RUN go get . && CGO_ENABLED=1 go install -a -ldflags '-s -w' -tags static && \
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
