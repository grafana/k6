package simple

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"time"
)

var (
	MetricReqs          = stats.New("http_reqs", stats.Counter)
	MetricReqDuration   = stats.New("http_req_duration", stats.Trend, stats.Time)
	MetricReqBlocked    = stats.New("http_req_blocked", stats.Trend, stats.Time)
	MetricReqLookingUp  = stats.New("http_req_looking_up", stats.Trend, stats.Time)
	MetricReqConnecting = stats.New("http_req_connecting", stats.Trend, stats.Time)
	MetricReqSending    = stats.New("http_req_sending", stats.Trend, stats.Time)
	MetricReqWaiting    = stats.New("http_req_waiting", stats.Trend, stats.Time)
	MetricReqReceiving  = stats.New("http_req_receiving", stats.Trend, stats.Time)
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
		Runner:    r,
		URLString: r.URL.String(),
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
	Runner   *Runner
	ID       int64
	IDString string

	URLString string
	Request   *http.Request
	Client    *http.Client

	tracer *lib.Tracer
	cTrace *httptrace.ClientTrace
}

func (u *VU) RunOnce(ctx context.Context) ([]stats.FatSample, error) {
	resp, err := u.Client.Do(u.Request.WithContext(httptrace.WithClientTrace(ctx, u.cTrace)))
	if err != nil {
		u.tracer.Done()
		return nil, err
	}

	_, _ = io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
	trail := u.tracer.Done()

	tags := map[string]string{
		"vu":     u.IDString,
		"method": "GET",
		"url":    u.URLString,
	}

	t := time.Now()
	return []stats.FatSample{
		stats.FatSample{
			Metric: MetricReqs,
			Sample: stats.Sample{Time: t, Tags: tags, Value: 1},
		},
		stats.FatSample{
			Metric: MetricReqDuration,
			Sample: stats.Sample{Time: t, Tags: tags, Value: float64(trail.Duration)},
		},
		stats.FatSample{
			Metric: MetricReqBlocked,
			Sample: stats.Sample{Time: t, Tags: tags, Value: float64(trail.Blocked)},
		},
		stats.FatSample{
			Metric: MetricReqLookingUp,
			Sample: stats.Sample{Time: t, Tags: tags, Value: float64(trail.LookingUp)},
		},
		stats.FatSample{
			Metric: MetricReqConnecting,
			Sample: stats.Sample{Time: t, Tags: tags, Value: float64(trail.Connecting)},
		},
		stats.FatSample{
			Metric: MetricReqSending,
			Sample: stats.Sample{Time: t, Tags: tags, Value: float64(trail.Sending)},
		},
		stats.FatSample{
			Metric: MetricReqWaiting,
			Sample: stats.Sample{Time: t, Tags: tags, Value: float64(trail.Waiting)},
		},
		stats.FatSample{
			Metric: MetricReqReceiving,
			Sample: stats.Sample{Time: t, Tags: tags, Value: float64(trail.Receiving)},
		},
	}, nil
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	u.IDString = strconv.FormatInt(id, 10)
	return nil
}
