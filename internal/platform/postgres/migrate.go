package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate uses a dedicated connection pool: schema changes never run implicitly
// during API startup. Goose serializes deployers with a session advisory lock.
func Migrate(ctx context.Context, databaseURL, command string) (int64, error) {
	if command != "up" && command != "status" && command != "down" {
		return 0, fmt.Errorf("unsupported migration command")
	}
	pool, err := Open(ctx, databaseURL, Options{MaxConns: 2, ConnectTimeout: 5 * time.Second})
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	db.SetMaxOpenConns(2)
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return 0, err
	}
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 30))
	if err != nil {
		return 0, err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files, goose.WithSessionLocker(locker), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return 0, err
	}
	switch command {
	case "up":
		_, err = provider.Up(ctx)
	case "down":
		_, err = provider.Down(ctx)
	}
	if err != nil {
		return 0, err
	}
	return provider.GetDBVersion(ctx)
}
