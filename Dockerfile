FROM golang:1.7

RUN curl -sL https://deb.nodesource.com/setup_6.x | bash && \
	apt-get install -y nodejs && \
	apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .
RUN npm -g install ember-cli bower && \
	make web && pwd && rm -r web/tmp web/node_modules web/bower_components && \
	go get . && go install . && rm -rf $GOPATH/lib $GOPATH/pkg && \
	(cd $GOPATH/src && ls | grep -v github | xargs rm -r) && \
	(cd $GOPATH/src/github.com && ls | grep -v loadimpact | xargs rm -r)

ENV K6_ADDRESS 0.0.0.0:6565
ENTRYPOINT ["k6"]
