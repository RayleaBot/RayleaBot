package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

type Runnable interface {
	Run(context.Context) error
	Logger() *slog.Logger
}

func RunWithSignals(application Runnable) error {
	ctx, stop := SignalContext()
	defer stop()

	return application.Run(ctx)
}

func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
