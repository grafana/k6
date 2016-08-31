package simple

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"
)

type Runner struct {
	URL       *url.URL
	Transport *http.Transport
}

func New(rawurl string) (*Runner, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	return &Runner{
		URL: u,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:        math.MaxInt32,
			MaxIdleConnsPerHost: math.MaxInt32,
		},
	}, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	tracer := &lib.Tracer{}

	return &VU{
		Runner: r,
		Request: &http.Request{
			Method: "GET",
			URL:    r.URL,
		},
		Client: &http.Client{
			Transport: r.Transport,
		},
		tracer: tracer,
		cTrace: tracer.Trace(),
	}, nil
}

type VU struct {
	Runner *Runner
	ID     int64

	Request *http.Request
	Client  *http.Client

	tracer *lib.Tracer
	cTrace *httptrace.ClientTrace
}

func (u *VU) RunOnce(ctx context.Context) error {
	resp, err := u.Client.Do(u.Request.WithContext(httptrace.WithClientTrace(ctx, u.cTrace)))
	if err != nil {
		u.tracer.Done()
		return err
	}

	_, _ = io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
	trail := u.tracer.Done()

	log.WithFields(log.Fields{
		"duration":   trail.Duration,
		"blocked":    trail.Blocked,
		"looking_up": trail.LookingUp,
		"connecting": trail.Connecting,
		"sending":    trail.Sending,
		"waiting":    trail.Waiting,
		"receiving":  trail.Receiving,
		"reused":     trail.ConnReused,
		"addr":       trail.ConnRemoteAddr,
	}).Info("Request")

	return nil
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	return nil
}
