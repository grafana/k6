FROM golang:1.6
ADD . /go/src/github.com/loadimpact/speedboat
RUN go get github.com/loadimpact/speedboat/...
RUN go install github.com/loadimpact/speedboat
ENTRYPOINT ["/go/bin/speedboat"]
