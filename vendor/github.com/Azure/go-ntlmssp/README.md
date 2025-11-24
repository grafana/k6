# go-ntlmssp

[![Go Reference](https://pkg.go.dev/badge/github.com/Azure/go-ntlmssp.svg)](https://pkg.go.dev/github.com/Azure/go-ntlmssp) [![Test](https://github.com/Azure/go-ntlmssp/actions/workflows/test.yml/badge.svg)](https://github.com/Azure/go-ntlmssp/actions/workflows/test.yml)

Go package that provides NTLM/Negotiate authentication over HTTP

* NTLM protocol details from https://msdn.microsoft.com/en-us/library/cc236621.aspx
* NTLM over HTTP details from https://datatracker.ietf.org/doc/html/rfc4559
* Implementation hints from http://davenport.sourceforge.net/ntlm.html

This package only implements authentication, no key exchange or encryption. It
only supports Unicode (UTF16LE) encoding of protocol strings, no OEM encoding.
This package implements NTLMv2.

# Installation

To install the package, use `go get`:

```bash
go get github.com/Azure/go-ntlmssp
```

# Usage

```go
url, user, password := "http://www.example.com/secrets", "robpike", "pw123"
client := &http.Client{
  Transport: ntlmssp.Negotiator{
    RoundTripper: &http.Transport{},
  },
}

req, _ := http.NewRequest("GET", url, nil)
req.SetBasicAuth(user, password)
res, _ := client.Do(req)
```

-----
This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
