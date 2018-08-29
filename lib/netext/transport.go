package netext

import (
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/ThomsonReutersEikon/go-ntlm/ntlm"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
)

type Transport struct {
	*http.Transport
	options   lib.Options
	tags      map[string]string
	trail     *Trail
	tlsInfo   TLSInfo
	samplesCh chan<- stats.SampleContainer

	// ntlm authentication
	mu              sync.Mutex
	ntlmAuthCache   map[string]bool
	enableNtlmCache bool
}

func NewTransport(transport *http.Transport, samplesCh chan<- stats.SampleContainer, options lib.Options, tags map[string]string) *Transport {
	return &Transport{
		Transport:       transport,
		tags:            tags,
		options:         options,
		samplesCh:       samplesCh,
		ntlmAuthCache:   make(map[string]bool),
		enableNtlmCache: true,
	}
}

func (t *Transport) CloseIdleConnections() {
	t.enableNtlmCache = false
	t.Transport.CloseIdleConnections()
}

func (t *Transport) SetOptions(options lib.Options) {
	t.options = options
}

func (t *Transport) GetTrail() *Trail {
	return t.trail
}

func (t *Transport) TLSInfo() TLSInfo {
	return t.tlsInfo
}

func (t *Transport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	if t.Transport == nil {
		return nil, errors.New("no roundtrip defined")
	}
	tags := t.tags

	ctx := req.Context()
	tracer := Tracer{}
	reqWithTracer := req.WithContext(WithTracer(ctx, &tracer))

	roundTripFunc := t.Transport.RoundTrip

	if GetAuth(req.Context()) == "ntlm" && req.URL.User != nil {
		roundTripFunc = t.roundtripWithNTLM
	}

	resp, err := roundTripFunc(reqWithTracer)
	if err != nil {
		if t.options.SystemTags["error"] {
			tags["error"] = err.Error()
		}

		//TODO: expand/replace this so we can recognize the different non-HTTP
		// errors, probably by using a type switch for resErr
		if t.options.SystemTags["status"] {
			tags["status"] = "0"
		}
	} else {
		if t.options.SystemTags["url"] {
			tags["url"] = req.URL.String()
		}
		if t.options.SystemTags["status"] {
			tags["status"] = strconv.Itoa(resp.StatusCode)
		}
		if t.options.SystemTags["proto"] {
			tags["proto"] = resp.Proto
		}

		if resp.TLS != nil {
			tlsInfo, oscp := ParseTLSConnState(resp.TLS)
			if t.options.SystemTags["tls_version"] {
				tags["tls_version"] = tlsInfo.Version
			}
			if t.options.SystemTags["ocsp_status"] {
				tags["ocsp_status"] = oscp.Status
			}

			t.tlsInfo = tlsInfo
		}

	}
	trail := tracer.Done()

	if t.options.SystemTags["ip"] && trail.ConnRemoteAddr != nil {
		if ip, _, err := net.SplitHostPort(trail.ConnRemoteAddr.String()); err == nil {
			tags["ip"] = ip
		}
	}

	t.trail = trail

	trail.SaveSamples(stats.IntoSampleTags(&tags))
	stats.PushIfNotCancelled(ctx, t.samplesCh, trail)

	return resp, err
}

func (t *Transport) roundtripWithNTLM(req *http.Request) (res *http.Response, err error) {
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
	if _, ok := t.getAuthCache(req.URL.String()); t.enableNtlmCache && ok {
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

func (t *Transport) setAuthCache(key string, value bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.ntlmAuthCache[key] = value
}

func (t *Transport) getAuthCache(key string) (bool, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	value, ok := t.ntlmAuthCache[key]
	return value, ok
}
