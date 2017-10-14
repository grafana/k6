FROM node:8.7-alpine
FROM golang:1.9-alpine

COPY --from=0 / /
WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .
RUN apk upgrade --no-cache && \
	apk --no-cache add --virtual .build-deps make git && \
	yarn global add ember-cli bower && \
	make web && pwd && rm -r web/tmp web/node_modules web/bower_components && \
	go get . && go install . && rm -rf $GOPATH/lib $GOPATH/pkg && \
	(cd $GOPATH/src && ls | grep -v github | xargs rm -r) && \
	(cd $GOPATH/src/github.com && ls | grep -v loadimpact | xargs rm -r) && \
	apk del .build-deps

ENV K6_ADDRESS 0.0.0.0:6565
ENTRYPOINT ["k6"]
