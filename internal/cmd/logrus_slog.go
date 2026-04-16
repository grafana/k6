package cmd

import (
	"context"
	"log/slog"

	"github.com/sirupsen/logrus"
)

// logrusSlogHandler is a slog.Handler that forwards to a logrus logger.
// Attribute keys are namespaced with any active group path (e.g. "group.key").
type logrusSlogHandler struct {
	logger    *logrus.Logger //nolint:forbidigo
	attrs     []slog.Attr    // pre-qualified attrs (keys already include group prefix)
	groupPath string         // dot-joined group names from outer to inner, e.g. "a.b"
}

func newLogrusSlogHandler(logger *logrus.Logger) slog.Handler { //nolint:forbidigo
	return &logrusSlogHandler{logger: logger}
}

func (h *logrusSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	switch {
	case level >= slog.LevelError:
		return h.logger.IsLevelEnabled(logrus.ErrorLevel)
	case level >= slog.LevelWarn:
		return h.logger.IsLevelEnabled(logrus.WarnLevel)
	case level >= slog.LevelInfo:
		return h.logger.IsLevelEnabled(logrus.InfoLevel)
	default:
		return h.logger.IsLevelEnabled(logrus.DebugLevel)
	}
}

func (h *logrusSlogHandler) Handle(_ context.Context, r slog.Record) error {
	fields := make(logrus.Fields, len(h.attrs)+r.NumAttrs())
	for _, a := range h.attrs {
		fields[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		flattenAttr(h.groupPath, a, fields)
		return true
	})

	entry := h.logger.WithFields(fields)
	switch {
	case r.Level >= slog.LevelError:
		entry.Error(r.Message)
	case r.Level >= slog.LevelWarn:
		entry.Warn(r.Message)
	case r.Level >= slog.LevelInfo:
		entry.Info(r.Message)
	default:
		entry.Debug(r.Message)
	}
	return nil
}

func (h *logrusSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Flatten and pre-qualify keys now so Handle doesn't repeat the work.
	fields := make(logrus.Fields, len(attrs))
	for _, a := range attrs {
		flattenAttr(h.groupPath, a, fields)
	}
	qualified := make([]slog.Attr, 0, len(fields))
	for k, v := range fields {
		qualified = append(qualified, slog.Any(k, v))
	}
	combined := make([]slog.Attr, len(h.attrs)+len(qualified))
	copy(combined, h.attrs)
	copy(combined[len(h.attrs):], qualified)
	return &logrusSlogHandler{logger: h.logger, attrs: combined, groupPath: h.groupPath}
}

func (h *logrusSlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	path := name
	if h.groupPath != "" {
		path = h.groupPath + "." + name
	}
	return &logrusSlogHandler{logger: h.logger, attrs: h.attrs, groupPath: path}
}

// flattenAttr writes a into fields with keys qualified by prefix.
// If a is a group-kind attr, it recurses into its children.
func flattenAttr(prefix string, a slog.Attr, fields logrus.Fields) {
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	if a.Value.Kind() == slog.KindGroup {
		for _, child := range a.Value.Group() {
			flattenAttr(key, child, fields)
		}
		return
	}
	if fields != nil {
		fields[key] = a.Value.Any()
	}
}
