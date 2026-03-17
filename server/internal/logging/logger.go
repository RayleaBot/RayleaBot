package logging

import (
	"fmt"
	"log/slog"
	"os"
)

func Bootstrap() *slog.Logger {
	return newLogger(slog.LevelInfo)
}

func New(levelName string) (*slog.Logger, error) {
	level, err := parseLevel(levelName)
	if err != nil {
		return nil, err
	}

	return newLogger(level), nil
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
	return slog.New(
		slog.NewJSONHandler(
			os.Stdout,
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
