package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testingStub struct {
	calls   int
	request Request
	err     error
}

func (s *testingStub) Enqueue(_ context.Context, r Request) (Receipt, error) {
	s.calls++
	s.request = r
	return Receipt{ID: "test-id", Status: "queued", DeliveryStatus: "pending"}, s.err
}
func (s *testingStub) Get(_ context.Context, scope Scope, id string) (Receipt, error) {
	s.calls++
	s.request.Scope = scope
	return Receipt{ID: id, Status: "accepted", DeliveryStatus: "delivered"}, s.err
}

func TestTestingRoutes(t *testing.T) {
	token := strings.Repeat("a", 32)
	body := `{"scope":"tenant","tenant_id":"demo","idempotency_key":"key","recipient":"test@example.com","template":"invitation.v1","expires_at":"2030-01-01T00:00:00Z","data":{"name":"Test"}}`
	for _, tc := range []struct {
		name, method, path, body, token string
		err                             error
		want, calls                     int
	}{
		{name: "unauthorized", method: "POST", path: "/testing/notifications", body: body, want: 401},
		{name: "wrong token", method: "POST", path: "/testing/notifications", body: body, token: "wrong", want: 401},
		{name: "enqueue", method: "POST", path: "/testing/notifications", body: body, token: token, want: 202, calls: 1},
		{name: "unknown field", method: "POST", path: "/testing/notifications", body: `{"unexpected":true}`, token: token, want: 400},
		{name: "trailing JSON", method: "POST", path: "/testing/notifications", body: body + ` {}`, token: token, want: 400},
		{name: "oversized", method: "POST", path: "/testing/notifications", body: strings.Repeat(" ", 33<<10) + body, token: token, want: 400},
		{name: "conflict", method: "POST", path: "/testing/notifications", body: body, token: token, err: ErrConflict, want: 409, calls: 1},
		{name: "invalid", method: "POST", path: "/testing/notifications", body: body, token: token, err: ErrInvalid, want: 400, calls: 1},
		{name: "status", method: "GET", path: "/testing/notifications/test-id?scope=tenant&tenant_id=demo", token: token, want: 200, calls: 1},
		{name: "scope not found", method: "GET", path: "/testing/notifications/test-id?scope=tenant&tenant_id=demo", token: token, err: ErrNotFound, want: 404, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &testingStub{err: tc.err}
			mux := http.NewServeMux()
			RegisterTestingRoutes(mux, stub, token)
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != tc.want || stub.calls != tc.calls {
				t.Fatalf("status=%d calls=%d; want %d %d", w.Code, stub.calls, tc.want, tc.calls)
			}
			if tc.calls > 0 && stub.request.Scope != TenantScope("demo") {
				t.Fatalf("scope not forwarded: %+v", stub.request.Scope)
			}
		})
	}
}

func TestTestingConfigGate(t *testing.T) {
	t.Setenv("NOTIFICATION_ENABLED", "false")
	t.Setenv("NOTIFICATION_TEST_API_ENABLED", "true")
	t.Setenv("NOTIFICATION_TEST_API_TOKEN", strings.Repeat("a", 32))
	if _, err := LoadConfig(); err == nil {
		t.Fatal("test API must require notification module enabled")
	}
	t.Setenv("NOTIFICATION_ENABLED", "true")
	t.Setenv("BREVO_API_KEY", "provider-key")
	t.Setenv("NOTIFICATION_SENDER_EMAIL", "sender@example.com")
	t.Setenv("NOTIFICATION_SENDER_NAME", "Test")
	t.Setenv("BREVO_WEBHOOK_TOKEN", strings.Repeat("b", 32))
	if _, err := LoadConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOTIFICATION_TEST_API_TOKEN", strings.Repeat("b", 32))
	if _, err := LoadConfig(); err == nil {
		t.Fatal("must reject reused webhook token")
	}
	t.Setenv("NOTIFICATION_TEST_API_TOKEN", "short")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("must reject short token")
	}
}
