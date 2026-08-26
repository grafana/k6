package flog

import (
	"net/url"
	"strings"
)

// RandResourceURI generates a random resource URI
func (f *Flog) RandResourceURI() string {
	var uri string
	num := f.gofakeit.Number(1, 4)
	for i := 0; i < num; i++ {
		uri += "/" + url.QueryEscape(f.gofakeit.BS())
	}
	uri = strings.ToLower(uri)
	return uri
}

// RandAuthUserID generates a random auth user id
func (f *Flog) RandAuthUserID() string {
	candidates := []string{"-", strings.ToLower(f.gofakeit.Username())}
	return candidates[f.rand.Intn(2)]
}

// RandHTTPVersion returns a random http version
func (f *Flog) RandHTTPVersion() string {
	versions := []string{"HTTP/1.0", "HTTP/1.1", "HTTP/2.0"}
	return versions[f.rand.Intn(3)]
}
