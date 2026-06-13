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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return application.Run(ctx)
}
