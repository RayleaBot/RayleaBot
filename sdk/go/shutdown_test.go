package rayleabot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

func TestShutdownCancellationIsNotLoggedAsHandlerFailure(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	t.Cleanup(func() {
		// Closing both sides also releases a blocked fixture peer on failure.
		_ = inputReader.Close()
		_ = inputWriter.Close()
		_ = outputReader.Close()
		_ = outputWriter.Close()
	})
	var diagnostics bytes.Buffer
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(t.Context(), Options{Stdin: inputReader, Stdout: outputWriter, Logger: slog.New(slog.NewJSONHandler(&diagnostics, nil))}, HandlerFunc(func(ctx context.Context, _ *EventContext) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}))
	}()
	encoder, decoder := json.NewEncoder(inputWriter), json.NewDecoder(outputReader)
	writeFrame(t, encoder, protocolFrame{ProtocolVersion: "2", Type: "init", Timezone: "Asia/Shanghai", PluginID: "fixture", RequestID: "init", Concurrency: 1})
	var ack protocolFrame
	decodeFrame(t, decoder, &ack)
	writeEvent(t, encoder, "fixture-event", "fixture")
	<-started
	writeFrame(t, encoder, protocolFrame{Type: "shutdown", RequestID: "stop", Reason: "stop"})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("normal shutdown emitted an error: %s", diagnostics.String())
	}
}
