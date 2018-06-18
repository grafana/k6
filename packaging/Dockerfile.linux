FROM golang:1.10.2-stretch

RUN curl https://glide.sh/get | sh

WORKDIR /go/src/github.com/loadimpact/k6

RUN go get github.com/mh-cbon/go-bin-deb \
  && cd /go/src/github.com/mh-cbon/go-bin-deb \
  && glide install \
  && go install

RUN go get github.com/mh-cbon/go-bin-rpm \
  && cd /go/src/github.com/mh-cbon/go-bin-rpm \
  && glide install \
  && go install

 RUN apt-get update -y && apt-get install -y fakeroot rpm
