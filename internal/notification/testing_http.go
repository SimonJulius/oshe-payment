package notification

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

type testingService interface {
	Enqueue(context.Context, Request) (Receipt, error)
	Get(context.Context, Scope, string) (Receipt, error)
}

// RegisterTestingRoutes exposes privileged manual testing operations only when
// explicitly called by the composition root. Never enable these in production.
func RegisterTestingRoutes(mux *http.ServeMux, service testingService, token string) {
	expected := sha256.Sum256([]byte("Bearer " + token))
	protect := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			actual := sha256.Sum256([]byte(r.Header.Get("Authorization")))
			if len(token) < 32 || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	respond := func(w http.ResponseWriter, receipt Receipt, err error, status int) {
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalid):
				http.Error(w, "invalid notification request", 400)
			case errors.Is(err, ErrConflict):
				http.Error(w, "idempotency key conflict", 409)
			case errors.Is(err, ErrNotFound):
				http.Error(w, "notification not found", 404)
			default:
				http.Error(w, "notification storage unavailable", 503)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": receipt.ID, "status": receipt.Status, "delivery_status": receipt.DeliveryStatus})
	}
	mux.HandleFunc("POST /testing/notifications", protect(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Scope          string            `json:"scope"`
			TenantID       string            `json:"tenant_id"`
			IdempotencyKey string            `json:"idempotency_key"`
			Recipient      string            `json:"recipient"`
			Template       string            `json:"template"`
			Data           map[string]string `json:"data"`
			ExpiresAt      time.Time         `json:"expires_at"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		defer r.Body.Close()
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			http.Error(w, "invalid JSON request", 400)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(w, "invalid JSON request", 400)
			return
		}
		receipt, err := service.Enqueue(r.Context(), Request{Scope: Scope{Kind: body.Scope, TenantID: body.TenantID}, IdempotencyKey: body.IdempotencyKey, Recipient: body.Recipient, Template: body.Template, Data: body.Data, ExpiresAt: body.ExpiresAt})
		respond(w, receipt, err, http.StatusAccepted)
	}))
	mux.HandleFunc("GET /testing/notifications/{id}", protect(func(w http.ResponseWriter, r *http.Request) {
		receipt, err := service.Get(r.Context(), Scope{Kind: r.URL.Query().Get("scope"), TenantID: r.URL.Query().Get("tenant_id")}, r.PathValue("id"))
		respond(w, receipt, err, http.StatusOK)
	}))
}
