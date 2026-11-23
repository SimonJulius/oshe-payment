// Package notification owns durable email delivery. Callers supply authorized
// scope and recipient; invitation creation and token validation belong to Auth.
package notification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalid  = errors.New("invalid notification request")
	ErrConflict = errors.New("idempotency key already used for a different request")
	ErrNotFound = errors.New("notification not found")
)

type Scope struct {
	Kind     string
	TenantID string
}

func PlatformScope() Scope        { return Scope{Kind: "platform"} }
func TenantScope(id string) Scope { return Scope{Kind: "tenant", TenantID: id} }
func (s Scope) valid() bool {
	return (s.Kind == "platform" && s.TenantID == "") || (s.Kind == "tenant" && strings.TrimSpace(s.TenantID) != "" && len(s.TenantID) <= 128)
}

type Request struct {
	Scope          Scope
	IdempotencyKey string
	Recipient      string
	Template       string
	Data           map[string]string
	ExpiresAt      time.Time
}

// Receipt deliberately omits recipient, template data, and invitation secrets.
type Receipt struct{ ID, Status, DeliveryStatus string }

type Service struct{ store *Store }

func NewService(store *Store) *Service { return &Service{store: store} }
func (s *Service) Enqueue(ctx context.Context, r Request) (Receipt, error) {
	return s.enqueue(ctx, s.store.pool, r)
}

// EnqueueTx lets Auth commit its invitation and the email request atomically.
// The caller owns commit/rollback and must use a transaction from the same DB.
func (s *Service) EnqueueTx(ctx context.Context, tx pgx.Tx, r Request) (Receipt, error) {
	return s.enqueue(ctx, tx, r)
}
func (s *Service) enqueue(ctx context.Context, db querier, r Request) (Receipt, error) {
	if !r.Scope.valid() || strings.TrimSpace(r.IdempotencyKey) == "" || len(r.IdempotencyKey) > 128 || !validEmail(r.Recipient) || r.ExpiresAt.IsZero() {
		return Receipt{}, ErrInvalid
	}
	// Copy caller data; callers may reuse the map after enqueueing.
	if len(r.Data) != 4 {
		return Receipt{}, ErrInvalid
	}
	dataCopy := make(map[string]string, 5)
	for k, v := range r.Data {
		dataCopy[k] = v
	}
	dataCopy["expires_at"] = r.ExpiresAt.UTC().Format(time.RFC3339)
	r.Data = dataCopy
	if _, err := render(r.Template, r.Data); err != nil {
		return Receipt{}, err
	}
	// PostgreSQL stores microseconds; canonicalize for repeatable request hashes.
	r.ExpiresAt = r.ExpiresAt.UTC().Truncate(time.Microsecond)
	raw, err := json.Marshal(r)
	if err != nil {
		return Receipt{}, ErrInvalid
	}
	sum := sha256.Sum256(raw)
	data, _ := json.Marshal(r.Data)
	return s.store.enqueue(ctx, db, r, data, hex.EncodeToString(sum[:]))
}
func (s *Service) Get(ctx context.Context, scope Scope, id string) (Receipt, error) {
	if !scope.valid() || !validID(id) {
		return Receipt{}, ErrInvalid
	}
	var r Receipt
	err := s.store.pool.QueryRow(ctx, `SELECT id::text,status,delivery_status FROM notification.messages WHERE id=$1 AND scope=$2 AND tenant_id=$3`, id, scope.Kind, scope.TenantID).Scan(&r.ID, &r.Status, &r.DeliveryStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}
func validEmail(v string) bool {
	a, e := mail.ParseAddress(v)
	return e == nil && a.Address == v && len(v) <= 254
}
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func validID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
