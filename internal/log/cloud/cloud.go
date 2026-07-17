// Package cloudlog implements a logrus hook that pushes k6's own logs to the
// Grafana Cloud k6 logs backend for 'k6 cloud run --local-execution'.
//
// The package lives in directory internal/log/cloud but is named cloudlog
// (not cloud) so that call sites which also import internal/secretsource/cloud
// do not have a package-name collision.
//
// It takes a plain Config struct and does not import cloudapi or
// internal/cloudapi/provisioning (avoiding an import cycle); the cmd layer
// translates the provisioning response or environment into Config.
package cloudlog

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/v2/internal/log"
)

// bufferCap bounds the pre-configuration entry buffer. It matches the loki
// hook's own channel capacity so Fire stays non-blocking under the same load.
const bufferCap = 1000

// testRunIDLabel is the stream label the backend requires on every push: a
// push whose stream lacks it, or carries a value not matching the token's run,
// is rejected with 401.
const testRunIDLabel = "test_run_id"

// Config is the log-push configuration, translated by the cmd layer from the
// provisioning response or the environment. It intentionally depends on no
// cloudapi or provisioning types.
type Config struct {
	PushURL       string        // loki push endpoint (empty => unconfigured)
	Token         string        // scoped test-run token, sent as Authorization: Bearer
	TestRunID     string        // added as the required test_run_id stream label
	Level         string        // minimum level; "" => all levels
	Limit         int           // max entries per push period
	PushPeriod    time.Duration // how often batches are flushed
	MsgMaxSize    int           // per-message truncation size
	AllowedLabels []string      // stream labels to keep; empty => keep all
}

// Pusher is a logrus AsyncHook that buffers log entries until SetConfig
// supplies the push URL and scoped token (available only after provisioning),
// then forwards buffered and subsequent entries to the cloud through an
// internal/log loki hook. It mirrors the cloud secret source's pre-register +
// SetConfig + lazy-init pattern.
type Pusher struct {
	fallbackLogger logrus.FieldLogger
	cfg            atomic.Pointer[Config]
	ready          chan struct{} // closed once by SetConfig when configured
	readyOnce      sync.Once
	buf            chan *logrus.Entry
	dropped        atomic.Int64
}

// Compile-time check that Pusher is usable as a logrus async hook.
var _ log.AsyncHook = (*Pusher)(nil)

// New creates a Pusher that buffers entries until SetConfig configures it.
func New(fallbackLogger logrus.FieldLogger) *Pusher {
	return &Pusher{
		fallbackLogger: fallbackLogger,
		ready:          make(chan struct{}),
		buf:            make(chan *logrus.Entry, bufferCap),
	}
}

// SetConfig stores the push configuration and signals readiness once. It is
// safe to call before or after Listen starts. An empty PushURL, TestRunID, or
// Token leaves the Pusher unconfigured (it keeps buffering and never pushes),
// since the backend requires all three.
func (p *Pusher) SetConfig(c Config) {
	if c.PushURL == "" || c.TestRunID == "" || c.Token == "" {
		if p.fallbackLogger != nil {
			p.fallbackLogger.Debug("cloud log push not configured: missing push URL, test run ID, or token")
		}
		return
	}
	p.cfg.Store(&c)
	p.readyOnce.Do(func() { close(p.ready) })
}

// Levels reports all levels; the underlying loki hook re-filters by its own
// configured level.
func (p *Pusher) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire buffers the entry without blocking. If the buffer is full the entry is
// dropped and counted, so the logging path is never blocked.
func (p *Pusher) Fire(entry *logrus.Entry) error {
	select {
	case p.buf <- entry:
	default:
		p.dropped.Add(1)
	}
	return nil
}

// Listen blocks until SetConfig has configured the Pusher or ctx is done. If
// ctx fires first (e.g. --no-cloud-logs or an early exit) it returns without
// pushing. Once configured it builds the loki hook and forwards buffered and
// subsequent entries to it, stopping when ctx is cancelled. It returns only
// after the loki hook's own final flush has completed.
func (p *Pusher) Listen(ctx context.Context) {
	select {
	case <-p.ready:
	case <-ctx.Done():
		return
	}

	c := p.cfg.Load()

	lokiHook, err := log.NewLokiHook(p.fallbackLogger, log.LokiHookOptions{
		Addr:          c.PushURL,
		Level:         c.Level,
		Limit:         c.Limit,
		PushPeriod:    c.PushPeriod,
		MsgMaxSize:    c.MsgMaxSize,
		AllowedLabels: allowedLabelsWithTestRunID(c.AllowedLabels),
		Labels:        [][2]string{{testRunIDLabel, c.TestRunID}},
		Headers:       [][2]string{{"Authorization", "Bearer " + c.Token}},
	})
	if err != nil {
		if p.fallbackLogger != nil {
			p.fallbackLogger.WithError(err).Error("cloud log push disabled: could not build the loki hook")
		}
		return
	}

	lokiDone := make(chan struct{})
	go func() {
		lokiHook.Listen(ctx)
		close(lokiDone)
	}()

	// Single consumer of buf, forwarding to the loki hook (the single
	// producer stays Fire). lokiHook.Fire is a blocking send, so once ctx is
	// cancelled we must stop forwarding: the loki hook's Listen has exited and
	// no longer drains its channel. Wait for its final flush before returning
	// so callers (loggersWg) don't tear down mid-flush.
	for {
		select {
		case e := <-p.buf:
			_ = lokiHook.Fire(e)
		case <-ctx.Done():
			<-lokiDone
			return
		}
	}
}

// allowedLabelsWithTestRunID ensures the required test_run_id label survives
// the loki hook's label filtering, which drops any label not in the allow-list
// when that list is non-empty. An empty list keeps all labels, so it is
// returned unchanged.
func allowedLabelsWithTestRunID(allowed []string) []string {
	if len(allowed) == 0 {
		return allowed
	}
	if slices.Contains(allowed, testRunIDLabel) {
		return allowed
	}
	return append(slices.Clone(allowed), testRunIDLabel)
}
