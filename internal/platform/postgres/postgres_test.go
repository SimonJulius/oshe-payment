package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRejectInvalidConfiguration(t *testing.T) {
	for _, url := range []string{"", "postgres://user:secret@host:bad/db"} {
		_, err := Open(context.Background(), url)
		if err == nil {
			t.Fatal("invalid URL accepted")
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("credentials exposed")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Open(ctx, "postgres://localhost:1/test?sslmode=disable"); err == nil {
		t.Fatal("cancelled connection succeeded")
	}
}

// TEST_DATABASE_URL must point to an empty disposable database. This test rolls
// back the foundation migration and deliberately blocks a rollback.
func TestIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to an empty disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, cmd := range []string{"up", "up", "status"} {
		v, err := Migrate(ctx, url, cmd)
		if err != nil || v != 1 {
			t.Fatalf("%s: version=%d err=%v", cmd, v, err)
		}
	}
	var timezone string
	if err := pool.QueryRow(ctx, "SHOW timezone").Scan(&timezone); err != nil || timezone != "UTC" {
		t.Fatalf("timezone=%s err=%v", timezone, err)
	}
	// A dependent object must prevent destructive rollback, including version updates.
	if _, err := pool.Exec(ctx, "CREATE TABLE platform.rollback_guard (id bigint PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(ctx, url, "down"); err == nil {
		t.Fatal("rollback deleted dependent objects")
	}
	if v, err := Migrate(ctx, url, "status"); err != nil || v != 1 {
		t.Fatalf("failed rollback changed version: %d %v", v, err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE platform.rollback_guard"); err != nil {
		t.Fatal(err)
	}
	if v, err := Migrate(ctx, url, "down"); err != nil || v != 0 {
		t.Fatalf("rollback: %d %v", v, err)
	}
	if _, err := Migrate(ctx, url, "up"); err != nil {
		t.Fatal(err)
	}
	// Duplicate runners serialize and both observe the same version.
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := Migrate(ctx, url, "up"); results <- err }()
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}
