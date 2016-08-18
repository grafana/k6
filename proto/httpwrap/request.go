package httpwrap

import (
	"context"
	"github.com/loadimpact/speedboat/stats"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
)

type Params struct {
	TakeSample bool
	KeepBody   bool
}

func Do(ctx context.Context, client *http.Client, req *http.Request, params Params) (*http.Response, []byte, stats.Sample, error) {
	var t Tracer
	if params.TakeSample {
		trace := t.MakeClientTrace()
		ctx = httptrace.WithClientTrace(ctx, &trace)
	}

	res, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, nil, stats.Sample{}, err
	}

	var body []byte
	if params.KeepBody {
		body, err = ioutil.ReadAll(res.Body)
	} else {
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
	}

	var sample stats.Sample
	if params.TakeSample {
		t.RequestDone()
		sample.Tags = stats.Tags{
			"proto":  res.Proto,
			"method": req.Method,
			"url":    req.URL.String(),
			"status": res.StatusCode,
		}
		sample.Values = stats.Values{
			"duration":     float64(t.Duration),
			"ttfb":         float64(t.TimeToFirstByte),
			"lookup":       float64(t.TimeForDNS),
			"connect":      float64(t.TimeForConnect),
			"send":         float64(t.TimeForWriteHeaders + t.TimeForWriteBody),
			"send_headers": float64(t.TimeForWriteHeaders),
			"send_body":    float64(t.TimeForWriteBody),
			"wait":         float64(t.TimeWaiting),
			"receive":      float64(t.Duration - t.TimeToFirstByte),
		}
	}

	return res, body, sample, err
}
