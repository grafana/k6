# go-http-digest-client

[![Build Status](https://travis-ci.org/Soontao/goHttpDigestClient.svg?branch=master)](https://travis-ci.org/Soontao/goHttpDigestClient) [![Coverage Status](https://coveralls.io/repos/github/Soontao/goHttpDigestClient/badge.svg?branch=master)](https://coveralls.io/github/Soontao/goHttpDigestClient?branch=master)

Library just for http digest auth, and refer RFC-2617

## install

get -u -v github.com/Soontao/goHttpDigestClient

## usage

```go
func TestClientAuthorize(t *testing.T) {
  req, err := http.NewRequest("GET", testDigestAuthServerURL, nil)
  if err != nil {
    t.Fatal(err)
  }
  opt := &ClientOption{username: testServerUsername, password: testServerPassword}
  res, err := DefaultClient.Do(req, opt)
}
```

## todo 

* [ ] if option in Client, only need 1 request get challenge