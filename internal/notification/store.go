package notification

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }
func (s *Store) enqueue(ctx context.Context, q querier, r Request, data []byte, hash string) (Receipt, error) {
	var out Receipt
	var storedHash string
	err := q.QueryRow(ctx, `INSERT INTO notification.messages (id,scope,tenant_id,idempotency_key,request_hash,recipient,template,data,expires_at)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
 ON CONFLICT (scope,tenant_id,idempotency_key) DO UPDATE SET idempotency_key=notification.messages.idempotency_key
 RETURNING id::text,status,delivery_status,request_hash`, newID(), r.Scope.Kind, r.Scope.TenantID, r.IdempotencyKey, hash, r.Recipient, r.Template, data, r.ExpiresAt).Scan(&out.ID, &out.Status, &out.DeliveryStatus, &storedHash)
	if err != nil {
		return Receipt{}, err
	}
	if storedHash != hash {
		return Receipt{}, ErrConflict
	}
	return out, nil
}

type job struct {
	ID, AttemptID, Recipient, Template string
	Data                               map[string]string
	Number                             int
}

// Claim and attempt creation commit before external I/O. Expired leases are
// quarantined as unknown, never blindly resent after a worker crash.
func (s *Store) claim(ctx context.Context) (job, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return job{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `WITH stale AS (
 UPDATE notification.messages SET status='unknown',lease_until=NULL,updated_at=now()
 WHERE status='processing' AND lease_until<now() RETURNING id)
 UPDATE notification.attempts SET status='unknown',error_code='lease_expired',finished_at=now()
 WHERE notification_id IN (SELECT id FROM stale) AND status='processing'`)
	if err != nil {
		return job{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE notification.messages SET status='expired',updated_at=now() WHERE status='queued' AND expires_at<=now()`)
	if err != nil {
		return job{}, err
	}
	var j job
	var data []byte
	err = tx.QueryRow(ctx, `SELECT id::text,recipient,template,data,attempt_count FROM notification.messages
 WHERE status='queued' AND next_attempt_at<=now() AND expires_at>now()
 ORDER BY next_attempt_at,created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&j.ID, &j.Recipient, &j.Template, &data, &j.Number)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, tx.Commit(ctx)
	}
	if err != nil {
		return j, err
	}
	if err = json.Unmarshal(data, &j.Data); err != nil {
		return j, err
	}
	j.Number++
	j.AttemptID = newID()
	_, err = tx.Exec(ctx, `UPDATE notification.messages SET status='processing',attempt_count=$2,lease_until=now()+interval '1 minute',updated_at=now() WHERE id=$1`, j.ID, j.Number)
	if err != nil {
		return j, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO notification.attempts(id,notification_id,number,status) VALUES($1,$2,$3,'processing')`, j.AttemptID, j.ID, j.Number)
	if err != nil {
		return j, err
	}
	return j, tx.Commit(ctx)
}

type outcome struct {
	Status, MessageID, Code string
	RetryAfter              time.Duration
}

func (s *Store) finish(ctx context.Context, j job, o outcome) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var state string
	if err = tx.QueryRow(ctx, `SELECT status FROM notification.messages WHERE id=$1 FOR UPDATE`, j.ID).Scan(&state); err != nil {
		return err
	}
	// A webhook may have already established acceptance, or lease recovery may
	// have quarantined the job. A late explicit response can still resolve unknown.
	var attemptState string
	if err = tx.QueryRow(ctx, `SELECT status FROM notification.attempts WHERE id=$1 FOR UPDATE`, j.AttemptID).Scan(&attemptState); err != nil {
		return err
	}
	if attemptState == "accepted" {
		return tx.Commit(ctx)
	}
	if o.Status == "retry" && j.Number >= 5 {
		o.Status = "failed"
		o.Code = "retry_exhausted"
	}
	_, err = tx.Exec(ctx, `UPDATE notification.attempts SET status=$2,message_id=NULLIF($3,''),error_code=NULLIF($4,''),finished_at=now() WHERE id=$1`, j.AttemptID, o.Status, o.MessageID, o.Code)
	if err != nil {
		return err
	}
	state = o.Status
	if state == "retry" {
		state = "queued"
	}
	_, err = tx.Exec(ctx, `UPDATE notification.messages SET status=$2,lease_until=NULL,next_attempt_at=now()+($3 * interval '1 second'),updated_at=now() WHERE id=$1`, j.ID, state, o.RetryAfter.Seconds())
	if err != nil {
		return err
	}
	if o.MessageID != "" {
		if err = s.reconcile(ctx, tx, j.AttemptID, j.ID, o.MessageID, j.Recipient); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// reconcile also handles webhooks that arrived before provider acceptance was saved.
func (s *Store) reconcile(ctx context.Context, tx pgx.Tx, attemptID, notificationID, messageID, recipient string) error {
	_, err := tx.Exec(ctx, `UPDATE notification.events SET attempt_id=$1 WHERE attempt_id IS NULL AND message_id=$2 AND recipient=$3`, attemptID, messageID, recipient)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `WITH best AS (
 SELECT CASE kind WHEN 'spam' THEN 'complained' WHEN 'hard_bounce' THEN 'bounced' WHEN 'invalid_email' THEN 'bounced' WHEN 'blocked' THEN 'blocked' WHEN 'error' THEN 'blocked' WHEN 'delivered' THEN 'delivered' ELSE 'deferred' END AS state,
 CASE kind WHEN 'spam' THEN 4 WHEN 'hard_bounce' THEN 3 WHEN 'invalid_email' THEN 3 WHEN 'blocked' THEN 3 WHEN 'error' THEN 3 WHEN 'delivered' THEN 2 ELSE 1 END AS rank,occurred_at
 FROM notification.events WHERE attempt_id=$1 AND kind IN ('spam','hard_bounce','invalid_email','blocked','error','delivered','deferred','soft_bounce')
 ORDER BY rank DESC,occurred_at DESC LIMIT 1)
 UPDATE notification.messages m SET delivery_status=best.state,delivery_rank=best.rank,delivery_at=best.occurred_at,updated_at=now()
 FROM best WHERE m.id=$2 AND (best.rank>m.delivery_rank OR (best.rank=m.delivery_rank AND best.occurred_at>=m.delivery_at))`, attemptID, notificationID)
	return err
}

// Revisit unmatched events in case webhook receipt and acceptance committed in
// opposite transactions without the custom correlation header being present.
func (s *Store) reconcilePending(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT e.kind,e.recipient,e.message_id,e.occurred_at
 FROM notification.events e JOIN notification.attempts a ON a.message_id=e.message_id
 JOIN notification.messages m ON m.id=a.notification_id AND m.recipient=e.recipient
 WHERE e.attempt_id IS NULL ORDER BY e.received_at LIMIT 100`)
	if err != nil {
		return err
	}
	var events []webhookEvent
	for rows.Next() {
		var e webhookEvent
		if err := rows.Scan(&e.Kind, &e.Email, &e.MessageID, &e.Timestamp); err != nil {
			rows.Close()
			return err
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range events {
		if err := s.recordEvent(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
