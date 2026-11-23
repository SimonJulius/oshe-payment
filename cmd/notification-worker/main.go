package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/SimonJulius/oshe-payment/internal/config"
	"github.com/SimonJulius/oshe-payment/internal/notification"
	"github.com/SimonJulius/oshe-payment/internal/platform/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	nc, err := notification.LoadConfig()
	if err != nil {
		return err
	}
	if !nc.Enabled {
		return errors.New("set NOTIFICATION_ENABLED=true to run notification worker")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(ctx, cfg.DatabaseURL, postgres.Options{MaxConns: cfg.DatabaseMaxConns, ConnectTimeout: cfg.DatabaseConnectTimeout})
	if err != nil {
		return errors.New("notification database startup failed")
	}
	defer pool.Close()
	slog.Info("notification worker started")
	return notification.NewWorker(notification.NewStore(pool), notification.NewBrevo(nc.APIKey, nc.SenderEmail, nc.SenderName)).Run(ctx)
}
