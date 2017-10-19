FROM golang:1.9-alpine

WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .
RUN apk --no-cache add --virtual .build-deps make git build-base && \
	go get . && go install . && rm -rf $GOPATH/lib $GOPATH/pkg && \
	(cd $GOPATH/src && ls | grep -v github | xargs rm -r) && \
	(cd $GOPATH/src/github.com && ls | grep -v loadimpact | xargs rm -r) && \
	apk del .build-deps

ENTRYPOINT ["k6"]
