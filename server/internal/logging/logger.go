package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

const (
	requestIDLogKey  = "request_id"
	defaultRequestID = "system"
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
		newRequestIDHandler(slog.NewJSONHandler(
			writer,
			&slog.HandlerOptions{
				Level:       level,
				ReplaceAttr: replaceAttr,
			},
		)),
	)
}

func newLoggerWithLevelVar(writer io.Writer, levelVar *slog.LevelVar) *slog.Logger {
	return slog.New(
		newRequestIDHandler(slog.NewJSONHandler(
			writer,
			&slog.HandlerOptions{
				Level:       levelVar,
				ReplaceAttr: replaceAttr,
			},
		)),
	)
}

type requestIDHandler struct {
	next         slog.Handler
	hasRequestID bool
}

func newRequestIDHandler(next slog.Handler) slog.Handler {
	return requestIDHandler{next: next}
}

func (h requestIDHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h requestIDHandler) Handle(ctx context.Context, record slog.Record) error {
	if !h.hasRequestID && !recordHasAttr(record, requestIDLogKey) {
		record.AddAttrs(slog.String(requestIDLogKey, defaultRequestID))
	}
	return h.next.Handle(ctx, record)
}

func (h requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return requestIDHandler{
		next:         h.next.WithAttrs(attrs),
		hasRequestID: h.hasRequestID || attrsContainKey(attrs, requestIDLogKey),
	}
}

func (h requestIDHandler) WithGroup(name string) slog.Handler {
	return requestIDHandler{
		next:         h.next.WithGroup(name),
		hasRequestID: h.hasRequestID,
	}
}

func recordHasAttr(record slog.Record, key string) bool {
	found := false
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key {
			found = true
			return false
		}
		return true
	})
	return found
}

func attrsContainKey(attrs []slog.Attr, key string) bool {
	for _, attr := range attrs {
		if attr.Key == key {
			return true
		}
	}
	return false
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

var logIDSequence atomic.Uint64

func generateLogID() string {
	return fmt.Sprintf("log_%d_%06d", time.Now().UTC().UnixNano(), logIDSequence.Add(1))
}

func generateBootID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("boot_%d_%06d", time.Now().UTC().UnixNano(), logIDSequence.Add(1))
	}
	return "boot_" + hex.EncodeToString(bytes[:])
}
