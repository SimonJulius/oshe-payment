package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SimonJulius/oshe-payment/internal/config"
	"github.com/SimonJulius/oshe-payment/internal/platform/postgres"
)

func main() {
	allowDown := flag.Bool("allow-down", false, "explicitly allow rollback of one migration")
	flag.Parse()
	if flag.NArg() != 1 || (flag.Arg(0) != "up" && flag.Arg(0) != "status" && flag.Arg(0) != "down") {
		slog.Error("usage: go run ./cmd/migrate [-allow-down] up|status|down")
		os.Exit(1)
	}
	if flag.Arg(0) == "down" && !*allowDown {
		slog.Error("rollback requires -allow-down; review data loss risk first")
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	version, err := postgres.Migrate(ctx, cfg.DatabaseURL, flag.Arg(0))
	if err != nil {
		slog.Error("database migration failed; inspect database logs for details")
		os.Exit(1)
	}
	slog.Info("database migration command complete", "command", flag.Arg(0), "version", version)
}
