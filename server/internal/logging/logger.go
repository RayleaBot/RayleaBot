package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// LevelController allows dynamic log level changes at runtime.
type LevelController struct {
	levelVar slog.LevelVar
}

// SetLevel changes the active log level. Returns an error for unsupported levels.
func (lc *LevelController) SetLevel(levelName string) error {
	level, err := parseLevel(levelName)
	if err != nil {
		return err
	}
	lc.levelVar.Set(level)
	return nil
}

// Level returns the current log level name.
func (lc *LevelController) Level() string {
	switch lc.levelVar.Level() {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return "info"
	}
}

func Bootstrap() *slog.Logger {
	return newLogger(slog.LevelInfo)
}

func New(levelName string) (*slog.Logger, error) {
	logger, _, _, err := NewWithStreamAndController(levelName, nil)
	return logger, err
}

// NewWithStream creates a logger with a management log stream. It returns a
// nil LevelController; use NewWithStreamAndController for dynamic level control.
func NewWithStream(levelName string, redactText func(string) string) (*slog.Logger, *Stream, error) {
	logger, stream, _, err := NewWithStreamAndController(levelName, redactText)
	return logger, stream, err
}

// NewWithStreamAndController creates a logger with a management log stream and
// a LevelController that allows changing the log level at runtime.
func NewWithStreamAndController(levelName string, redactText func(string) string) (*slog.Logger, *Stream, *LevelController, error) {
	level, err := parseLevel(levelName)
	if err != nil {
		return nil, nil, nil, err
	}

	lc := &LevelController{}
	lc.levelVar.Set(level)

	stream := NewStream(32)
	stream.SetBootID(generateBootID())
	writer := NewSummaryWriter(os.Stdout, stream, redactText)
	logger := newLoggerWithLevelVar(writer, &lc.levelVar)
	return logger, stream, lc, nil
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
				Level:       level,
				ReplaceAttr: replaceAttr,
			},
		),
	)
}

func newLoggerWithLevelVar(writer io.Writer, levelVar *slog.LevelVar) *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(
			writer,
			&slog.HandlerOptions{
				Level:       levelVar,
				ReplaceAttr: replaceAttr,
			},
		),
	)
}

func replaceAttr(_ []string, attr slog.Attr) slog.Attr {
	switch attr.Key {
	case slog.TimeKey:
		if attr.Value.Kind() == slog.KindTime {
			attr.Key = "ts"
		}
	case slog.MessageKey:
		attr.Key = "msg"
	}
	return attr
}
