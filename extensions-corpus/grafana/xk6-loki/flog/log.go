package flog

import (
	"fmt"
	"strings"
	"time"
)

const (
	// ApacheCommonLog : {host} {user-identifier} {auth-user-id} [{datetime}] "{method} {request} {protocol}" {response-code} {bytes}
	ApacheCommonLog = "%s - %s [%s] \"%s %s %s\" %d %d"
	// ApacheCombinedLog : {host} {user-identifier} {auth-user-id} [{datetime}] "{method} {request} {protocol}" {response-code} {bytes} "{referrer}" "{agent}"
	ApacheCombinedLog = "%s - %s [%s] \"%s %s %s\" %d %d \"%s\" \"%s\""
	// ApacheErrorLog : [{timestamp}] [{module}:{severity}] [pid {pid}:tid {thread-id}] [client %{client}:{port}] %{message}
	ApacheErrorLog = "[%s] [%s:%s] [pid %d:tid %d] [client %s:%d] %s"
	// RFC3164Log : <priority>{timestamp} {hostname} {application}[{pid}]: {message}
	RFC3164Log = "<%d>%s %s %s[%d]: %s"
	// RFC5424Log : <priority>{version} {iso-timestamp} {hostname} {application} {pid} {message-id} {structured-data} {message}
	RFC5424Log = "<%d>%d %s %s %s %d ID%d %s %s"
	// CommonLogFormat : {host} {user-identifier} {auth-user-id} [{datetime}] "{method} {request} {protocol}" {response-code} {bytes}
	CommonLogFormat = "%s - %s [%s] \"%s %s %s\" %d %d"
	// JSONLogFormat : {"host": "{host}", "user-identifier": "{user-identifier}", "datetime": "{datetime}", "method": "{method}", "request": "{request}", "protocol": "{protocol}", "status", {status}, "bytes": {bytes}, "referer": "{referer}"}
	JSONLogFormat = `{"host":"%s", "user-identifier":"%s", "datetime":"%s", "method": "%s", "request": "%s", "protocol":"%s", "status":%d, "bytes":%d, "referer": "%s"}`
	// LogFmtLogFormat : host={host} user={user-identifier} timestamp={datetime} method={method} request="{request}" protocol={protocol} status={status} bytes={bytes} referer="{referer}"
	LogFmtLogFormat = `host="%s" user=%s timestamp=%s method=%s request="%s" protocol=%s status=%d bytes=%d referer="%s"`
)

// NewApacheCommonLog creates a log string with apache common log format
func (f *Flog) NewApacheCommonLog(t time.Time) string {
	return fmt.Sprintf(
		ApacheCommonLog,
		f.gofakeit.IPv4Address(),
		f.RandAuthUserID(),
		t.Format(Apache),
		f.gofakeit.HTTPMethod(),
		f.RandResourceURI(),
		f.RandHTTPVersion(),
		f.gofakeit.HTTPStatusCodeSimple(),
		f.gofakeit.Number(0, 30000),
	)
}

// NewApacheCombinedLog creates a log string with apache combined log format
func (f *Flog) NewApacheCombinedLog(t time.Time) string {
	return fmt.Sprintf(
		ApacheCombinedLog,
		f.gofakeit.IPv4Address(),
		f.RandAuthUserID(),
		t.Format(Apache),
		f.gofakeit.HTTPMethod(),
		f.RandResourceURI(),
		f.RandHTTPVersion(),
		f.gofakeit.HTTPStatusCodeSimple(),
		f.gofakeit.Number(30, 100000),
		f.gofakeit.URL(),
		f.gofakeit.UserAgent(),
	)
}

// NewApacheErrorLog creates a log string with apache error log format
func (f *Flog) NewApacheErrorLog(t time.Time) string {
	return fmt.Sprintf(
		ApacheErrorLog,
		t.Format(ApacheError),
		f.gofakeit.Word(),
		f.gofakeit.LogLevel("apache"),
		f.gofakeit.Number(1, 10000),
		f.gofakeit.Number(1, 10000),
		f.gofakeit.IPv4Address(),
		f.gofakeit.Number(1, 65535),
		f.gofakeit.HackerPhrase(),
	)
}

// NewRFC3164Log creates a log string with syslog (RFC3164) format
func (f *Flog) NewRFC3164Log(t time.Time) string {
	return fmt.Sprintf(
		RFC3164Log,
		f.gofakeit.Number(0, 191),
		t.Format(RFC3164),
		strings.ToLower(f.gofakeit.Username()),
		f.gofakeit.Word(),
		f.gofakeit.Number(1, 10000),
		f.gofakeit.HackerPhrase(),
	)
}

// NewRFC5424Log creates a log string with syslog (RFC5424) format
func (f *Flog) NewRFC5424Log(t time.Time) string {
	return fmt.Sprintf(
		RFC5424Log,
		f.gofakeit.Number(0, 191),
		f.gofakeit.Number(1, 3),
		t.Format(RFC5424),
		f.gofakeit.DomainName(),
		f.gofakeit.Word(),
		f.gofakeit.Number(1, 10000),
		f.gofakeit.Number(1, 1000),
		"-", // TODO: structured data
		f.gofakeit.HackerPhrase(),
	)
}

// NewCommonLogFormat creates a log string with common log format
func (f *Flog) NewCommonLogFormat(t time.Time) string {
	return fmt.Sprintf(
		CommonLogFormat,
		f.gofakeit.IPv4Address(),
		f.RandAuthUserID(),
		t.Format(CommonLog),
		f.gofakeit.HTTPMethod(),
		f.RandResourceURI(),
		f.RandHTTPVersion(),
		f.gofakeit.HTTPStatusCodeSimple(),
		f.gofakeit.Number(0, 30000),
	)
}

// NewJSONLogFormat creates a log string with json log format
func (f *Flog) NewJSONLogFormat(t time.Time) string {
	return fmt.Sprintf(
		JSONLogFormat,
		f.gofakeit.IPv4Address(),
		f.RandAuthUserID(),
		t.Format(CommonLog),
		f.gofakeit.HTTPMethod(),
		f.RandResourceURI(),
		f.RandHTTPVersion(),
		f.gofakeit.HTTPStatusCodeSimple(),
		f.gofakeit.Number(0, 30000),
		f.gofakeit.URL(),
	)
}

// NewLogFmtLogFormat creates a log string with logfmt log format
func (f *Flog) NewLogFmtLogFormat(t time.Time) string {
	return fmt.Sprintf(
		LogFmtLogFormat,
		f.gofakeit.IPv4Address(),
		f.RandAuthUserID(),
		t.Format(RFC5424),
		f.gofakeit.HTTPMethod(),
		f.RandResourceURI(),
		f.RandHTTPVersion(),
		f.gofakeit.HTTPStatusCodeSimple(),
		f.gofakeit.Number(0, 30000),
		f.gofakeit.URL(),
	)
}
