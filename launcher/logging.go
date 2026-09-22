package main

import (
	"io"
	"log/slog"
)

func newLauncherLogger(writer io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey && attr.Value.Kind() == slog.KindTime {
				attr.Key = "ts"
				attr.Value = slog.StringValue(attr.Value.Time().UTC().Format("2006-01-02T15:04:05.000000000Z"))
			}
			return attr
		},
	})).With("component", "launcher")
}
