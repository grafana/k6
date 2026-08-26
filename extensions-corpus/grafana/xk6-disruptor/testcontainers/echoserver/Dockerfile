FROM golang:1.25@sha256:8c945d3e25320e771326dafc6fb72ecae5f87b0f29328cbbd87c4dff506c9135

WORKDIR /echoserver
COPY echoserver.go .
RUN go build -o /bin/echoserver echoserver.go

ENTRYPOINT [ "/bin/echoserver" ]
