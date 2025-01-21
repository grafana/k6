package websockets

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"

	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
	httpModule "go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

// wsParams represent the parameters bag for websocket
type wsParams struct {
	headers           http.Header
	cookieJar         *cookiejar.Jar
	tagsAndMeta       *metrics.TagsAndMeta
	enableCompression bool
	subprocotols      []string
}

// buildParams builds WebSocket params and configure some of them
func buildParams(state *lib.State, rt *sobek.Runtime, raw sobek.Value) (*wsParams, error) {
	tagsAndMeta := state.Tags.GetCurrentValues()

	parsed := &wsParams{
		headers:     make(http.Header),
		cookieJar:   state.CookieJar,
		tagsAndMeta: &tagsAndMeta,
	}

	parsed.headers.Set("User-Agent", state.Options.UserAgent.String)

	if raw == nil || sobek.IsUndefined(raw) {
		return parsed, nil
	}

	params := raw.ToObject(rt)
	for _, k := range params.Keys() {
		switch k {
		case "headers":
			headersV := params.Get(k)
			if sobek.IsUndefined(headersV) || sobek.IsNull(headersV) {
				continue
			}
			headersObj := headersV.ToObject(rt)
			if headersObj == nil {
				continue
			}
			for _, key := range headersObj.Keys() {
				parsed.headers.Set(key, headersObj.Get(key).String())
			}
		case "tags":
			if err := common.ApplyCustomUserTags(rt, parsed.tagsAndMeta, params.Get(k)); err != nil {
				return nil, fmt.Errorf("invalid WebSocket tags option: %w", err)
			}
		case "jar":
			jarV := params.Get(k)
			if sobek.IsUndefined(jarV) || sobek.IsNull(jarV) {
				continue
			}
			if v, ok := jarV.Export().(*httpModule.CookieJar); ok {
				parsed.cookieJar = v.Jar
			}
		case "compression":
			// deflate compression algorithm is supported - as defined in RFC7692
			// compression here relies on the implementation in gorilla/websocket package, usage is
			// experimental and may result in decreased performance. package supports
			// only "no context takeover" scenario

			algoString := strings.TrimSpace(params.Get(k).ToString().String())
			if algoString != "deflate" {
				return nil, fmt.Errorf("unsupported compression algorithm '%s', supported algorithm is 'deflate'", algoString)
			}

			parsed.enableCompression = true
		default:
			return nil, fmt.Errorf("unknown WebSocket's option %s", k)
		}
	}

	return parsed, nil
}
