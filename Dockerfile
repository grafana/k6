FROM golang:1.7

# Add the sources
WORKDIR /go/src/github.com/loadimpact/k6
ADD . .

# Build the binary
RUN go get ./... && go install ./... && rm -rf /go/lib

# Build the web UI + JS runner
RUN curl -sL https://deb.nodesource.com/setup_6.x | bash && \
	apt-get install -y nodejs && \
	npm -g install ember-cli bower && \
	cd web && \
	npm install && \
	bower install --allow-root && \
	ember build --env production && \
	rm -rf -- tmp node_modules bower_components && \
	cd .. && \
	cd js && \
	npm install && \
	apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /
ENTRYPOINT ["/go/bin/k6"]
