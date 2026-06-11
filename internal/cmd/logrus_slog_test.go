package cmd

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/sirupsen/logrus"
)

// captureLogrus returns a logrus logger that writes JSON to buf.
func captureLogrus(buf *bytes.Buffer) *logrus.Logger { //nolint:forbidigo
	l := logrus.New()
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.DebugLevel)
	l.SetOutput(buf)
	return l
}

func TestLogrusSlogHandler_Levels(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, `"level":"debug"`},
		{slog.LevelInfo, `"level":"info"`},
		{slog.LevelWarn, `"level":"warning"`},
		{slog.LevelError, `"level":"error"`},
	} {
		t.Run(tc.level.String(), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			logger := slog.New(newLogrusSlogHandler(captureLogrus(&buf)))
			logger.Log(context.Background(), tc.level, "msg")
			if !bytes.Contains(buf.Bytes(), []byte(tc.want)) {
				t.Errorf("expected %q in output %q", tc.want, buf.String())
			}
		})
	}
}

func TestLogrusSlogHandler_WithGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(newLogrusSlogHandler(captureLogrus(&buf)))

	// single group
	logger.WithGroup("provider").Info("msg", "key", "val")
	if !bytes.Contains(buf.Bytes(), []byte(`"provider.key":"val"`)) {
		t.Errorf("expected provider.key in output: %s", buf.String())
	}
	buf.Reset()

	// nested groups
	logger.WithGroup("a").WithGroup("b").Info("msg", "key", "val")
	if !bytes.Contains(buf.Bytes(), []byte(`"a.b.key":"val"`)) {
		t.Errorf("expected a.b.key in output: %s", buf.String())
	}
	buf.Reset()

	// WithAttrs after WithGroup
	sub := logger.WithGroup("g").With("pre", "v")
	sub.Info("msg", "post", "w")
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(`"g.pre":"v"`)) {
		t.Errorf("expected g.pre in output: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"g.post":"w"`)) {
		t.Errorf("expected g.post in output: %s", out)
	}
	buf.Reset()

	// empty group name is a no-op
	logger.WithGroup("").Info("msg", "key", "bare")
	if !bytes.Contains(buf.Bytes(), []byte(`"key":"bare"`)) {
		t.Errorf("expected bare key in output: %s", buf.String())
	}
}

func TestLogrusSlogHandler_GroupKindAttr(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(newLogrusSlogHandler(captureLogrus(&buf)))

	// slog.Group() produces a KindGroup attr — should be flattened
	logger.Info("msg", slog.Group("outer", slog.String("inner", "val")))
	if !bytes.Contains(buf.Bytes(), []byte(`"outer.inner":"val"`)) {
		t.Errorf("expected outer.inner in output: %s", buf.String())
	}
}
