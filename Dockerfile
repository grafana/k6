FROM golang:1.7

WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .
RUN curl -sL https://deb.nodesource.com/setup_6.x | bash && \
	apt-get install -y nodejs && \
	npm -g install ember-cli bower && \
	make web && rm -rf web/{node_modules,bower_components} && \
	go get . && go install . && rm -rf $GOPATH/lib && \
	apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENV K6_ADDRESS 0.0.0.0:6565
ENTRYPOINT ["k6"]
