package notification

import (
	"context"
	"log/slog"
	"time"
)

type Worker struct {
	store  *Store
	sender Sender
}

func NewWorker(store *Store, sender Sender) *Worker { return &Worker{store: store, sender: sender} }

// Run processes one message at a time. Run additional processes for concurrency;
// PostgreSQL SKIP LOCKED prevents two workers from claiming the same message.
func (w *Worker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		worked, err := w.Once(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("notification worker operation failed")
		}
		if !worked || err != nil {
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	}
	return nil
}
func (w *Worker) Once(ctx context.Context) (bool, error) {
	if err := w.store.reconcilePending(ctx); err != nil {
		return false, err
	}
	j, err := w.store.claim(ctx)
	if err != nil || j.ID == "" {
		return false, err
	}
	c, err := render(j.Template, j.Data)
	o := outcome{Status: "failed", Code: "template_invalid"}
	if err == nil {
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		o = w.sender.send(sendCtx, j, c)
		cancel()
	}
	// Persist a known result even when shutdown cancelled the send context.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err = w.store.finish(saveCtx, j, o)
	if err == nil {
		slog.Info("notification attempt completed", "notification_id", j.ID, "attempt", j.Number, "status", o.Status)
	}
	return true, err
}
