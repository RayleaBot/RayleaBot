package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

func Bootstrap() *slog.Logger {
	return newLogger(slog.LevelInfo)
}

func New(levelName string) (*slog.Logger, error) {
	logger, _, err := NewWithStream(levelName)
	return logger, err
}

func NewWithStream(levelName string) (*slog.Logger, *Stream, error) {
	level, err := parseLevel(levelName)
	if err != nil {
		return nil, nil, err
	}

	stream := NewStream(32)
	return newLoggerWithWriter(level, NewSummaryWriter(os.Stdout, stream)), stream, nil
}

func parseLevel(levelName string) (slog.Level, error) {
	switch levelName {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q", levelName)
	}
}

func newLogger(level slog.Level) *slog.Logger {
	return newLoggerWithWriter(level, os.Stdout)
}

func newLoggerWithWriter(level slog.Level, writer io.Writer) *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(
			writer,
			&slog.HandlerOptions{
				Level: level,
				ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
					switch attr.Key {
					case slog.TimeKey:
						attr.Key = "ts"
					case slog.MessageKey:
						attr.Key = "msg"
					}

					return attr
				},
			},
		),
	)
}
