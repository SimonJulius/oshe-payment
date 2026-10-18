package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	MaxConns       int32
	ConnectTimeout time.Duration
}

// Open establishes a bounded pool and verifies connectivity before returning.
// Defaults are intentionally small; budget total connections across all replicas.
func Open(ctx context.Context, databaseURL string, options ...Options) (*pgxpool.Pool, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("database URL is required")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	// Parsing errors may contain credentials; do not propagate the input.
	if err != nil {
		return nil, errors.New("invalid database connection configuration")
	}
	opt := Options{MaxConns: 10, ConnectTimeout: 5 * time.Second}
	if len(options) > 1 {
		return nil, errors.New("expected at most one postgres options value")
	}
	if len(options) == 1 {
		opt = options[0]
	}
	if opt.MaxConns < 1 || opt.ConnectTimeout <= 0 {
		return nil, errors.New("pool size and connect timeout must be positive")
	}
	cfg.MaxConns = opt.MaxConns
	cfg.MinConns = 0
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnLifetimeJitter = 5 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = time.Minute
	cfg.ConnConfig.ConnectTimeout = opt.ConnectTimeout
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	cfg.ConnConfig.RuntimeParams["application_name"] = "oshe-payment"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "30000"
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "5000"
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "30000"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	checkCtx, cancel := context.WithTimeout(ctx, opt.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(checkCtx); err != nil {
		pool.Close()
		// Connection errors can include hosts/usernames; callers should log a generic failure.
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}
