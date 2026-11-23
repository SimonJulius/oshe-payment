package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SimonJulius/oshe-payment/internal/apidocs"
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
	notificationConfig, err := notification.LoadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(ctx, cfg.DatabaseURL, postgres.Options{MaxConns: cfg.DatabaseMaxConns, ConnectTimeout: cfg.DatabaseConnectTimeout})
	if err != nil {
		return errors.New("database startup failed; inspect database logs for details")
	}
	defer pool.Close()
	mux := http.NewServeMux()
	apidocs.Register(mux)
	if notificationConfig.TestAPIEnabled {
		notification.RegisterTestingRoutes(mux, notification.NewService(notification.NewStore(pool)), notificationConfig.TestAPIToken)
	}
	if notificationConfig.Enabled {
		mux.Handle("POST /webhooks/brevo", notification.WebhookHandler(notification.NewStore(pool), notificationConfig.WebhookToken))
	}
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := pool.Ping(checkCtx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := &http.Server{Addr: cfg.Address(), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	slog.Info("API started", "address", cfg.Address())
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
	}
	return nil
}
