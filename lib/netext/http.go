package netext

import (
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/ThomsonReutersEikon/go-ntlm/ntlm"
	"github.com/pkg/errors"
)

type HTTPTransport struct {
	*http.Transport

	mu          sync.Mutex
	authCache   map[string]bool
	enableCache bool
}

func NewHTTPTransport(transport *http.Transport) *HTTPTransport {
	return &HTTPTransport{
		Transport:   transport,
		authCache:   make(map[string]bool),
		enableCache: true,
	}
}

func (t *HTTPTransport) CloseIdleConnections() {
	t.enableCache = false
	t.Transport.CloseIdleConnections()
}

func (t *HTTPTransport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	if t.Transport == nil {
		return nil, errors.New("no roundtrip defined")
	}

	// checking if the request needs ntlm authentication
	if GetAuth(req.Context()) == "ntlm" && req.URL.User != nil {
		return t.roundtripWithNTLM(req)
	}

	return t.Transport.RoundTrip(req)
}

func (t *HTTPTransport) roundtripWithNTLM(req *http.Request) (res *http.Response, err error) {
	rt := t.Transport

	username := req.URL.User.Username()
	password, _ := req.URL.User.Password()

	// Save request body
	body := bytes.Buffer{}
	if req.Body != nil {
		_, err = body.ReadFrom(req.Body)
		if err != nil {
			return nil, err
		}

		if err := req.Body.Close(); err != nil {
			return nil, err
		}
		req.Body = ioutil.NopCloser(bytes.NewReader(body.Bytes()))
	}

	// before making the request check if there is a cached authorization.
	if _, ok := t.getAuthCache(req.URL.String()); t.enableCache && ok {
		req.Header.Del("Authorization")
	} else {
		req.Header.Set("Authorization", "NTLM TlRMTVNTUAABAAAAB4IAAAAAAAAAAAAAAAAAAAAAAAAAAAAAMAAAAAAAMAA=")
	}

	res, err = rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusUnauthorized {
		return res, err
	}

	if _, err := io.Copy(ioutil.Discard, res.Body); err != nil {
		return nil, err
	}
	if err := res.Body.Close(); err != nil {
		return nil, err
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(body.Bytes()))

	// retrieve Www-Authenticate header from response
	ntlmChallenge := res.Header.Get("WWW-Authenticate")
	if !strings.HasPrefix(ntlmChallenge, "NTLM ") {
		return nil, errors.New("Invalid WWW-Authenticate header")
	}

	challengeBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ntlmChallenge, "NTLM "))
	if err != nil {
		return nil, err
	}

	session, err := ntlm.CreateClientSession(ntlm.Version2, ntlm.ConnectionlessMode)
	if err != nil {
		return nil, err
	}

	session.SetUserInfo(username, password, "")

	// parse NTLM challenge
	challenge, err := ntlm.ParseChallengeMessage(challengeBytes)
	if err != nil {
		return nil, err
	}

	err = session.ProcessChallengeMessage(challenge)
	if err != nil {
		return nil, err
	}

	// authenticate user
	authenticate, err := session.GenerateAuthenticateMessage()
	if err != nil {
		return nil, err
	}

	// set NTLM Authorization header
	header := "NTLM " + base64.StdEncoding.EncodeToString(authenticate.Bytes())
	req.Header.Set("Authorization", header)

	t.setAuthCache(req.URL.String(), true)

	return rt.RoundTrip(req)
}

func (t *HTTPTransport) setAuthCache(key string, value bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.authCache[key] = value
}

func (t *HTTPTransport) getAuthCache(key string) (bool, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	value, ok := t.authCache[key]
	return value, ok
}
