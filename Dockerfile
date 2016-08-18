FROM golang:1.7
WORKDIR /go/src/github.com/loadimpact/speedboat
ADD . .
RUN go get ./... && go install ./...
ENTRYPOINT ["/go/bin/speedboat"]
