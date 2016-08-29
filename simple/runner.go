package simple

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"net"
	"net/http"
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
		},
	}, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	return &VU{
		Runner: r,
		Request: http.Request{
			Method: "GET",
			URL:    r.URL,
		},
		Client: http.Client{
			Transport: r.Transport,
		},
	}, nil
}

type VU struct {
	Runner *Runner
	ID     int64

	Request http.Request
	Client  http.Client
}

func (u *VU) RunOnce(ctx context.Context) error {
	log.WithField("id", u.ID).Info("Running")
	return nil
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	return nil
}
