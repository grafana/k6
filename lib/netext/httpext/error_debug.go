package httpext

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/http/httputil"
	"slices"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/v2/internal/lib/netext/httpdebug"
	"go.k6.io/k6/v2/lib"
)

func logFailedRequest(
	ctx context.Context, state *lib.State, preq *ParsedHTTPRequest, requestID string,
	res *http.Response, resp *Response, resErr error,
) {
	if resErr != nil && ctx.Err() != nil {
		return
	}
	if state.Options.HTTPDebug.String != "error" {
		if resErr != nil {
			state.Logger.WithField("error", resErr).Warn("Request Failed")
		}
		return
	}
	if resErr == nil && !isFailedStatus(preq.ResponseCallback, resp.Status) {
		return
	}

	redact := func(dump []byte) []byte {
		if state.Options.HTTPDebugRedact.Bool {
			return httpdebug.RedactDump(dump)
		}
		return dump
	}

	meta := debugMeta(state, preq, requestID)
	details := "Request:\n" + string(redact(dumpRequest(ctx, state.Logger, preq)))
	if resErr != nil {
		meta["error"] = resErr
		meta["error_code"] = resp.ErrorCode
	} else {
		details += "\nResponse:\n" + string(redact(dumpResponse(state.Logger, res, resp.Body)))
	}
	state.Logger.Warn(DebugMessage("Request Failed", meta, details))
}

// dumpRequest dumps the request without being affected by ctx cancellation, so a request
// that has already finished can still be dumped.
func dumpRequest(ctx context.Context, logger logrus.FieldLogger, preq *ParsedHTTPRequest) []byte {
	clone := preq.Req.Clone(context.WithoutCancel(ctx))
	clone.Body = nil
	if preq.Req.GetBody != nil {
		body, err := preq.Req.GetBody()
		if err != nil {
			logger.Error(err)
		}
		clone.Body = body
	}
	if clone.Body == nil {
		clone.ContentLength = 0
	}

	dump, err := httputil.DumpRequestOut(clone, clone.Body != nil)
	if err != nil {
		logger.Error(err)
	}
	return bytes.TrimRight(bytes.ReplaceAll(dump, []byte("\r\n"), []byte{'\n'}), "\n")
}

func dumpResponse(logger logrus.FieldLogger, res *http.Response, body any) []byte {
	dump, err := httputil.DumpResponse(res, false)
	if err != nil {
		logger.Error(err)
	}
	dump = bytes.ReplaceAll(dump, []byte("\r\n"), []byte{'\n'})

	switch b := body.(type) {
	case []byte:
		dump = append(dump, b...)
	case string:
		dump = append(dump, b...)
	}
	return bytes.TrimRight(dump, "\n")
}

// debugMeta returns the metadata printed in --http-debug=error messages: the VU, the
// iteration, the request ID, and the request's tags and metadata.
func debugMeta(state *lib.State, preq *ParsedHTTPRequest, requestID string) map[string]any {
	meta := map[string]any{"vu": state.VUID, "iter": state.Iteration, "request_id": requestID}
	AddMissing(meta, preq.TagsAndMeta.Metadata, preq.TagsAndMeta.Tags.Map())
	return meta
}

// AddMissing copies each key from srcs into dst unless dst already has it. Earlier srcs
// take precedence over later ones.
func AddMissing(dst map[string]any, srcs ...map[string]string) {
	for _, src := range srcs {
		for k, v := range src {
			if _, ok := dst[k]; !ok {
				dst[k] = v
			}
		}
	}
}

func isFailedStatus(callback func(int) bool, status int) bool {
	return callback != nil && !callback(status)
}

func (res *Response) httpextResponse() *Response {
	return res
}

type responseHolder interface {
	httpextResponse() *Response
}

// DebugDump returns the request ID, tags and error code of v, along with a text dump of
// its request and response, if v is an HTTP response. ok is false otherwise.
func DebugDump(v any, redact bool) (fields map[string]string, dump string, ok bool) {
	holder, ok := v.(responseHolder)
	if !ok {
		return nil, "", false
	}
	res := holder.httpextResponse()
	if res == nil {
		return nil, "", false
	}
	if res.tags != nil || res.requestID != "" {
		fields = make(map[string]string, len(res.tags)+2)
		maps.Copy(fields, res.tags)
		if res.requestID != "" {
			fields["request_id"] = res.requestID
		}
		if res.ErrorCode != 0 {
			fields["error_code"] = strconv.Itoa(res.ErrorCode)
		}
	}
	return fields, res.debugDump(redact), true
}

func (res *Response) debugDump(redact bool) string {
	var req, resp strings.Builder

	if r := res.Request; r != nil {
		fmt.Fprintf(&req, "%s %s\n", r.Method, r.URL)
		for _, k := range slices.Sorted(maps.Keys(r.Headers)) {
			for _, v := range r.Headers[k] {
				if redact && httpdebug.IsSensitive(k) {
					v = httpdebug.RedactValue(k, v)
				}
				fmt.Fprintf(&req, "%s: %s\n", k, v)
			}
		}
		if r.Body != "" {
			fmt.Fprintf(&req, "\n%s", r.Body)
		}
	}

	if res.Error != "" {
		fmt.Fprintf(&resp, "error: %s\n", res.Error)
	}
	if res.Status != 0 {
		fmt.Fprintf(&resp, "%s %s\n", res.Proto, res.StatusText)
		for _, k := range slices.Sorted(maps.Keys(res.Headers)) {
			v := res.Headers[k]
			if redact && httpdebug.IsSensitive(k) {
				v = res.redactHeader(k, v)
			}
			fmt.Fprintf(&resp, "%s: %s\n", k, v)
		}
		var body string
		switch v := res.Body.(type) {
		case string:
			body = v
		case []byte:
			body = string(v)
		}
		if body != "" {
			fmt.Fprintf(&resp, "\n%s", body)
		}
	}

	return "Request:\n" + strings.TrimRight(req.String(), "\n") +
		"\nResponse:\n" + strings.TrimRight(resp.String(), "\n")
}

// redactHeader masks a sensitive response header. Several Set-Cookie headers are joined with ", "
// in Headers, and a comma can also be part of Expires, so the cookie names are taken from Cookies.
func (res *Response) redactHeader(name, value string) string {
	if !strings.EqualFold(name, "Set-Cookie") {
		return httpdebug.RedactValue(name, value)
	}
	if len(res.Cookies) == 0 {
		return httpdebug.Mask
	}
	names := slices.Sorted(maps.Keys(res.Cookies))
	pairs := make([]string, len(names))
	for i, n := range names {
		pairs[i] = n + "=" + httpdebug.Mask
	}
	return strings.Join(pairs, ", ")
}

// DebugMessage formats a debug log message: the title, then the non-empty meta values as
// sorted key=value pairs, then the details.
func DebugMessage(title string, meta map[string]any, details string) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\nContext:")
	for _, k := range slices.Sorted(maps.Keys(meta)) {
		v := fmt.Sprint(meta[k])
		if v == "" {
			continue
		}
		if strings.ContainsAny(v, " \t\n\"=") {
			v = strconv.Quote(v)
		}
		fmt.Fprintf(&b, " %s=%s", k, v)
	}
	b.WriteString("\n\n")
	b.WriteString(strings.TrimRight(details, "\n"))
	b.WriteString("\n\n")
	return b.String()
}
