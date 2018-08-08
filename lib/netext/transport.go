package netext

import (
	"net"
	"net/http"
	"strconv"

	"github.com/k0kubun/pp"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
)

type Transport struct {
	*http.Transport
	options   lib.Options
	samplesCh chan<- stats.SampleContainer
}

func NewTransport(transport *http.Transport, samplesCh chan<- stats.SampleContainer, options lib.Options) *Transport {
	return &Transport{
		Transport: transport,
		options:   options,
		samplesCh: samplesCh,
	}
}

func (t *Transport) SetOptions(options lib.Options) {
	t.options = options
}

func (t *Transport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	if t.Transport == nil {
		return nil, errors.New("no roundtrip defined")
	}
	ctx := req.Context()

	tags := t.options.RunTags.CloneTags()

	tracer := Tracer{}
	req.WithContext(WithTracer(ctx, &tracer))
	pp.Println(req.URL.String())
	resp, err := t.Transport.RoundTrip(req.WithContext(WithTracer(ctx, &tracer)))
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
	}
	trail := tracer.Done()
	if t.options.SystemTags["ip"] && trail.ConnRemoteAddr != nil {
		if ip, _, err := net.SplitHostPort(trail.ConnRemoteAddr.String()); err == nil {
			tags["ip"] = ip
		}
	}

	trail.SaveSamples(stats.IntoSampleTags(&tags))
	stats.PushIfNotCancelled(ctx, t.samplesCh, trail)

	return resp, err
}
