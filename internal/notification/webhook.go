package notification

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type webhookEvent struct {
	Kind          string `json:"event"`
	Email         string `json:"email"`
	MessageID     string `json:"message-id"`
	Timestamp     int64  `json:"ts_event"`
	SentTimestamp int64  `json:"ts"`
	Custom        string `json:"X-Mailin-custom"`
}

// WebhookHandler authenticates Brevo's configured bearer token. No tenant ID
// from the webhook is trusted: ownership is recovered from an existing attempt.
func WebhookHandler(store *Store, token string) http.Handler {
	expected := sha256.Sum256([]byte("Bearer " + token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		actual := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if token == "" || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		defer r.Body.Close()
		var e webhookEvent
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&e); err != nil {
			http.Error(w, "invalid webhook", http.StatusBadRequest)
			return
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			http.Error(w, "invalid webhook", http.StatusBadRequest)
			return
		}
		e.MessageID = normalizeMessageID(e.MessageID)
		if e.Timestamp == 0 {
			e.Timestamp = e.SentTimestamp
		}
		if !validEmail(e.Email) || e.MessageID == "" || len(e.MessageID) > 512 || e.Timestamp <= 0 || len(e.Custom) > 1024 {
			http.Error(w, "invalid webhook", http.StatusBadRequest)
			return
		}
		switch e.Kind {
		case "request", "sent", "delivered", "deferred", "soft_bounce", "hard_bounce", "blocked", "invalid_email", "error", "spam":
		default:
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := store.recordEvent(r.Context(), e); err != nil {
			http.Error(w, "webhook persistence unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
func (s *Store) recordEvent(ctx context.Context, e webhookEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Brevo's `id` is the webhook configuration ID, not a unique event ID.
	raw, _ := json.Marshal([]any{e.MessageID, e.Email, e.Kind, e.Timestamp})
	hash := sha256.Sum256(raw)
	key := hex.EncodeToString(hash[:])
	persist := func() error {
		_, err := tx.Exec(ctx, `INSERT INTO notification.events(event_key,message_id,recipient,kind,occurred_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, key, e.MessageID, e.Email, e.Kind, e.Timestamp)
		return err
	}
	var attemptID, notificationID string
	candidate := strings.TrimPrefix(e.Custom, "attempt_id:")
	if !strings.HasPrefix(e.Custom, "attempt_id:") || !validID(candidate) {
		candidate = "00000000-0000-0000-0000-000000000000"
	}
	err = tx.QueryRow(ctx, `SELECT a.id::text,a.notification_id::text FROM notification.attempts a JOIN notification.messages m ON m.id=a.notification_id
 WHERE m.recipient=$1 AND (a.message_id=$2 OR (a.id=$3 AND a.status IN ('processing','unknown','accepted') AND (a.message_id IS NULL OR a.message_id=$2))) LIMIT 1`, e.Email, e.MessageID, candidate).Scan(&attemptID, &notificationID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = persist(); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	// All paths lock message then attempt, avoiding deadlocks with worker completion.
	var state string
	if err = tx.QueryRow(ctx, `SELECT status FROM notification.messages WHERE id=$1 FOR UPDATE`, notificationID).Scan(&state); err != nil {
		return err
	}
	var messageID *string
	if err = tx.QueryRow(ctx, `SELECT message_id FROM notification.attempts WHERE id=$1 FOR UPDATE`, attemptID).Scan(&messageID); err != nil {
		return err
	}
	if messageID != nil && *messageID != e.MessageID {
		return fmt.Errorf("provider message correlation conflict")
	}
	_, err = tx.Exec(ctx, `UPDATE notification.attempts SET status='accepted',message_id=$2,error_code=NULL,finished_at=COALESCE(finished_at,now()) WHERE id=$1`, attemptID, e.MessageID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE notification.messages SET status='accepted',lease_until=NULL,updated_at=now() WHERE id=$1`, notificationID)
	if err != nil {
		return err
	}
	if err = persist(); err != nil {
		return err
	}
	if err = s.reconcile(ctx, tx, attemptID, notificationID, e.MessageID, e.Email); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
