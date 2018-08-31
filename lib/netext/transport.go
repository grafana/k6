package netext

import (
	"net"
	"net/http"
	"strconv"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
)

type Transport struct {
	http.RoundTripper
	options   *lib.Options
	tags      map[string]string
	trail     *Trail
	tlsInfo   TLSInfo
	samplesCh chan<- stats.SampleContainer
}

func NewTransport(transport http.RoundTripper, samplesCh chan<- stats.SampleContainer, options *lib.Options, tags map[string]string) *Transport {
	return &Transport{
		RoundTripper: transport,
		tags:         tags,
		options:      options,
		samplesCh:    samplesCh,
	}
}

func (t *Transport) SetOptions(options *lib.Options) {
	t.options = options
}

func (t *Transport) GetTrail() *Trail {
	return t.trail
}

func (t *Transport) TLSInfo() TLSInfo {
	return t.tlsInfo
}

func (t *Transport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	if t.RoundTripper == nil {
		return nil, errors.New("no roundtrip defined")
	}
	tags := t.tags

	ctx := req.Context()
	tracer := Tracer{}
	reqWithTracer := req.WithContext(WithTracer(ctx, &tracer))

	resp, err := t.RoundTripper.RoundTrip(reqWithTracer)
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
