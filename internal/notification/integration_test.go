package notification

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/SimonJulius/oshe-payment/internal/platform/postgres"
)

type senderFunc func(context.Context, job, content) outcome

func (f senderFunc) send(c context.Context, j job, b content) outcome { return f(c, j, b) }

// Use an empty disposable database, distinct from TEST_DATABASE_URL because
// packages run concurrently and the platform tests exercise migration rollback.
func TestPostgresIntegration(t *testing.T) {
	url := os.Getenv("NOTIFICATION_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set NOTIFICATION_TEST_DATABASE_URL to a disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if v, e := postgres.Migrate(ctx, url, "up"); e != nil || v != 2 {
		t.Fatalf("migrate: %d %v", v, e)
	}
	pool, e := postgres.Open(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	messageSuffix := newID()
	store := NewStore(pool)
	svc := NewService(store)
	enqueue := func(t *testing.T, scope Scope) Receipt {
		t.Helper()
		r, e := svc.Enqueue(ctx, invitation(scope, newID()))
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	get := func(t *testing.T, id string) Receipt {
		t.Helper()
		r, e := svc.Get(ctx, PlatformScope(), id)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	t.Run("scope and idempotency", func(t *testing.T) {
		request := invitation(TenantScope("tenant-a"), newID())
		first, e := svc.Enqueue(ctx, request)
		if e != nil {
			t.Fatal(e)
		}
		const n = 8
		errs := make(chan error, n)
		var wg sync.WaitGroup
		for range n {
			wg.Go(func() {
				r, e := svc.Enqueue(ctx, request)
				if e == nil && r.ID != first.ID {
					e = errors.New("duplicate notification")
				}
				errs <- e
			})
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			if e != nil {
				t.Fatal(e)
			}
		}
		request.Recipient = "other@example.com"
		if _, e := svc.Enqueue(ctx, request); !errors.Is(e, ErrConflict) {
			t.Fatalf("conflict: %v", e)
		}
		request.Scope = TenantScope("tenant-b")
		second, e := svc.Enqueue(ctx, request)
		if e != nil || second.ID == first.ID {
			t.Fatal("tenant key collision", e)
		}
		if _, e := svc.Get(ctx, TenantScope("tenant-b"), first.ID); !errors.Is(e, ErrNotFound) {
			t.Fatal("cross-tenant read", e)
		}
		if _, e := svc.Get(ctx, PlatformScope(), first.ID); !errors.Is(e, ErrNotFound) {
			t.Fatal("platform scope bypassed tenant ownership")
		}
		request.Scope = PlatformScope()
		third, e := svc.Enqueue(ctx, request)
		if e != nil || third.ID == second.ID {
			t.Fatal("platform key collision", e)
		}
		if _, e := pool.Exec(ctx, `UPDATE notification.messages SET status='expired' WHERE id IN ($1,$2,$3)`, first.ID, second.ID, third.ID); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("transaction rollback", func(t *testing.T) {
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		r, e := svc.EnqueueTx(ctx, tx, invitation(PlatformScope(), newID()))
		if e != nil {
			t.Fatal(e)
		}
		if e = tx.Rollback(ctx); e != nil {
			t.Fatal(e)
		}
		if _, e = svc.Get(ctx, PlatformScope(), r.ID); !errors.Is(e, ErrNotFound) {
			t.Fatal("email survived invitation rollback", e)
		}
	})
	t.Run("concurrent claims", func(t *testing.T) {
		r := enqueue(t, PlatformScope())
		jobs := make(chan job, 8)
		errs := make(chan error, 8)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() { j, e := store.claim(ctx); jobs <- j; errs <- e })
		}
		wg.Wait()
		close(jobs)
		close(errs)
		for e := range errs {
			if e != nil {
				t.Fatal(e)
			}
		}
		count := 0
		var claimed job
		for j := range jobs {
			if j.ID != "" {
				count++
				claimed = j
				if j.ID != r.ID {
					t.Fatal("unexpected job")
				}
			}
		}
		if count != 1 {
			t.Fatalf("claimed %d times", count)
		}
		if e := store.finish(ctx, claimed, outcome{Status: "failed", Code: "test"}); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("acceptance and webhook ordering", func(t *testing.T) {
		r := enqueue(t, PlatformScope())
		w := NewWorker(store, senderFunc(func(_ context.Context, j job, c content) outcome {
			if c.HTML == "" || c.Text == "" {
				t.Fatal("missing rendered content")
			}
			// A genuine delivery can race ahead of the HTTP response.
			e := store.recordEvent(ctx, webhookEvent{Kind: "delivered", Email: j.Recipient, MessageID: "early-message" + messageSuffix, Timestamp: 200, Custom: "attempt_id:" + j.AttemptID})
			if e != nil {
				t.Fatal(e)
			}
			return outcome{Status: "accepted", MessageID: "early-message" + messageSuffix}
		}))
		if worked, e := w.Once(ctx); e != nil || !worked {
			t.Fatal(worked, e)
		}
		if got := get(t, r.ID); got.Status != "accepted" || got.DeliveryStatus != "delivered" {
			t.Fatalf("%+v", got)
		}
		for _, kind := range []string{"delivered", "deferred", "hard_bounce", "delivered", "spam"} {
			if e := store.recordEvent(ctx, webhookEvent{Kind: kind, Email: "invitee@example.com", MessageID: "early-message" + messageSuffix, Timestamp: 200}); e != nil {
				t.Fatal(e)
			}
		}
		if got := get(t, r.ID); got.DeliveryStatus != "complained" {
			t.Fatalf("%+v", got)
		}
		var count int
		if e := pool.QueryRow(ctx, `SELECT count(*) FROM notification.events WHERE message_id=$1`, "early-message"+messageSuffix).Scan(&count); e != nil || count != 4 {
			t.Fatalf("dedup count %d %v", count, e)
		}
	})
	t.Run("unmatched webhook and recipient check", func(t *testing.T) {
		r := enqueue(t, PlatformScope())
		j, e := store.claim(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if e = store.recordEvent(ctx, webhookEvent{Kind: "delivered", Email: "wrong@example.com", MessageID: "unmatched" + messageSuffix, Timestamp: 201, Custom: "attempt_id:" + j.AttemptID}); e != nil {
			t.Fatal(e)
		}
		if got := get(t, r.ID); got.Status != "processing" {
			t.Fatal("wrong recipient matched")
		}
		if e = store.recordEvent(ctx, webhookEvent{Kind: "delivered", Email: j.Recipient, MessageID: "unmatched" + messageSuffix, Timestamp: 201}); e != nil {
			t.Fatal(e)
		}
		if e = store.finish(ctx, j, outcome{Status: "accepted", MessageID: "unmatched" + messageSuffix}); e != nil {
			t.Fatal(e)
		}
		if got := get(t, r.ID); got.DeliveryStatus != "delivered" {
			t.Fatal("unmatched event was lost")
		}
	})
	t.Run("concurrent webhook and acceptance", func(t *testing.T) {
		for range 12 {
			r := enqueue(t, PlatformScope())
			j, e := store.claim(ctx)
			if e != nil {
				t.Fatal(e)
			}
			messageID := newID() + "@brevo.test"
			errs := make(chan error, 2)
			go func() {
				errs <- store.recordEvent(ctx, webhookEvent{Kind: "delivered", Email: j.Recipient, MessageID: messageID, Timestamp: 202})
			}()
			go func() { errs <- store.finish(ctx, j, outcome{Status: "accepted", MessageID: messageID}) }()
			for range 2 {
				if e := <-errs; e != nil {
					t.Fatal(e)
				}
			}
			if e := store.reconcilePending(ctx); e != nil {
				t.Fatal(e)
			}
			if get(t, r.ID).DeliveryStatus != "delivered" {
				t.Fatal("concurrent event was lost")
			}
		}
	})
	t.Run("bounded rate limit retries", func(t *testing.T) {
		r := enqueue(t, PlatformScope())
		calls := 0
		w := NewWorker(store, senderFunc(func(context.Context, job, content) outcome {
			calls++
			return outcome{Status: "retry", Code: "provider_rate_limit"}
		}))
		for range 6 {
			if _, e := w.Once(ctx); e != nil {
				t.Fatal(e)
			}
		}
		if calls != 5 || get(t, r.ID).Status != "failed" {
			t.Fatalf("calls=%d receipt=%+v", calls, get(t, r.ID))
		}
	})
	t.Run("unknown is never blindly resent", func(t *testing.T) {
		r := enqueue(t, PlatformScope())
		calls := 0
		w := NewWorker(store, senderFunc(func(context.Context, job, content) outcome {
			calls++
			return outcome{Status: "unknown", Code: "provider_transport"}
		}))
		for range 2 {
			if _, e := w.Once(ctx); e != nil {
				t.Fatal(e)
			}
		}
		if calls != 1 || get(t, r.ID).Status != "unknown" {
			t.Fatal("unknown request retried")
		}
	})
	t.Run("crashed lease recovery", func(t *testing.T) {
		r := enqueue(t, PlatformScope())
		j, e := store.claim(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, `UPDATE notification.messages SET lease_until=now()-interval '1 second' WHERE id=$1`, r.ID); e != nil {
			t.Fatal(e)
		}
		next, e := store.claim(ctx)
		if e != nil || next.ID != "" || get(t, r.ID).Status != "unknown" {
			t.Fatal("crashed lease resent", e)
		}
		if e = store.recordEvent(ctx, webhookEvent{Kind: "delivered", Email: j.Recipient, MessageID: "late-message" + messageSuffix, Timestamp: 202, Custom: "attempt_id:" + j.AttemptID}); e != nil {
			t.Fatal(e)
		}
		if got := get(t, r.ID); got.Status != "accepted" || got.DeliveryStatus != "delivered" {
			t.Fatal("late event did not resolve crash")
		}
	})
	t.Run("expired invitation never sent", func(t *testing.T) {
		request := invitation(PlatformScope(), newID())
		request.ExpiresAt = time.Now().Add(-time.Hour)
		r, e := svc.Enqueue(ctx, request)
		if e != nil {
			t.Fatal(e)
		}
		w := NewWorker(store, senderFunc(func(context.Context, job, content) outcome { t.Fatal("expired email sent"); return outcome{} }))
		if _, e = w.Once(ctx); e != nil {
			t.Fatal(e)
		}
		if get(t, r.ID).Status != "expired" {
			t.Fatal("expiry not recorded")
		}
	})
}
