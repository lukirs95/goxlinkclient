package xlinkclient

import (
	"context"
	"log/slog"
)

type NullLogHandler struct{}

func (h *NullLogHandler) Enabled(context.Context, slog.Level) bool {
	return false
}

func (h *NullLogHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (h *NullLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *NullLogHandler) WithGroup(name string) slog.Handler {
	return h
}
