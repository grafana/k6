FROM golang:1.9-alpine

WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .
RUN apk --no-cache add --virtual .build-deps make git build-base && \
	go get . && go install . && rm -rf $GOPATH/pkg && \
	apk del .build-deps

ENTRYPOINT ["k6"]
