package modules

import (
	"context"
	"io"
	"log/slog"

	"github.com/sirupsen/logrus"
)

var extensionAPIDiscardLogger = slog.New(slog.NewTextHandler(io.Discard, nil)) //nolint:gochecknoglobals

// extensionAPISlogHandler adapts the stable slog logger exposed to extensions
// to k6's current Logrus-based logging implementation.
type extensionAPISlogHandler struct {
	logger    logrus.FieldLogger
	attrs     []slog.Attr
	groupPath string
}

func newExtensionAPISlogHandler(logger logrus.FieldLogger) slog.Handler {
	return &extensionAPISlogHandler{logger: logger}
}

func (h *extensionAPISlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	levelChecker, ok := h.logger.(interface{ IsLevelEnabled(logrus.Level) bool })
	return !ok || levelChecker.IsLevelEnabled(extensionAPILogrusLevel(level))
}

func (h *extensionAPISlogHandler) Handle(_ context.Context, record slog.Record) error {
	fields := make(logrus.Fields, len(h.attrs)+record.NumAttrs())
	for _, attr := range h.attrs {
		extensionAPIFlattenSlogAttr("", attr, fields)
	}
	record.Attrs(func(attr slog.Attr) bool {
		extensionAPIFlattenSlogAttr(h.groupPath, attr, fields)
		return true
	})

	entry := h.logger.WithFields(fields)
	switch extensionAPILogrusLevel(record.Level) {
	case logrus.ErrorLevel:
		entry.Error(record.Message)
	case logrus.WarnLevel:
		entry.Warn(record.Message)
	case logrus.InfoLevel:
		entry.Info(record.Message)
	default:
		entry.Debug(record.Message)
	}
	return nil
}

func (h *extensionAPISlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	combined = append(combined, h.attrs...)
	for _, attr := range attrs {
		if attr.Equal(slog.Attr{}) {
			continue
		}
		combined = append(combined, extensionAPIQualifySlogAttr(h.groupPath, attr))
	}
	return &extensionAPISlogHandler{logger: h.logger, attrs: combined, groupPath: h.groupPath}
}

func (h *extensionAPISlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	groupPath := name
	if h.groupPath != "" {
		groupPath = h.groupPath + "." + name
	}
	return &extensionAPISlogHandler{logger: h.logger, attrs: h.attrs, groupPath: groupPath}
}

func extensionAPILogrusLevel(level slog.Level) logrus.Level {
	switch {
	case level >= slog.LevelError:
		return logrus.ErrorLevel
	case level >= slog.LevelWarn:
		return logrus.WarnLevel
	case level >= slog.LevelInfo:
		return logrus.InfoLevel
	default:
		return logrus.DebugLevel
	}
}

func extensionAPIQualifySlogAttr(prefix string, attr slog.Attr) slog.Attr {
	if prefix == "" {
		return attr
	}
	return slog.Attr{Key: prefix + "." + attr.Key, Value: attr.Value}
}

func extensionAPIFlattenSlogAttr(prefix string, attr slog.Attr, fields logrus.Fields) {
	attr.Value = attr.Value.Resolve()
	key := attr.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	if attr.Value.Kind() == slog.KindGroup {
		for _, child := range attr.Value.Group() {
			extensionAPIFlattenSlogAttr(key, child, fields)
		}
		return
	}
	if key != "" {
		fields[key] = attr.Value.Any()
	}
}
