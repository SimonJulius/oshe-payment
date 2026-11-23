package notification

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func invitation(scope Scope, key string) Request {
	return Request{Scope: scope, IdempotencyKey: key, Recipient: "invitee@example.com", Template: InvitationTemplate, Data: map[string]string{"name": "Alex", "inviter": "Simon", "organization": "Example", "invitation_url": "https://example.com/invite?token=secret"}, ExpiresAt: time.Now().UTC().Truncate(time.Second).Add(time.Hour)}
}
func TestInvitationRendering(t *testing.T) {
	data := invitation(PlatformScope(), "a").Data
	data["expires_at"] = "2026-10-19T12:00:00Z"
	data["name"] = "<script>alert(1)</script>"
	c, err := render(InvitationTemplate, data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.HTML, "<script>") || !strings.Contains(c.HTML, "&lt;script&gt;") || !strings.Contains(c.Text, "<script>") {
		t.Fatal("HTML escaping or text rendering failed")
	}
	for _, url := range []string{"javascript:alert(1)", "http://example.com/invite", "https://user:pass@example.com/invite"} {
		data["invitation_url"] = url
		if _, err := render(InvitationTemplate, data); !errors.Is(err, ErrInvalid) {
			t.Fatal("unsafe URL accepted")
		}
	}
	if _, err := render("missing", data); !errors.Is(err, ErrInvalid) {
		t.Fatal("unknown template accepted")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBrevoContractAndOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"accepted", 201, `{"messageId":"<provider@example.com>"}`, "accepted"},
		{"bad response", 201, `{}`, "unknown"}, {"rate limited", 429, `{}`, "retry"}, {"server error", 503, `{}`, "unknown"}, {"rejected", 400, `{}`, "failed"}, {"timeout", 408, `{}`, "unknown"}, {"transport", 0, ``, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBrevo("test-key", "sender@example.com", "OshePayment")
			b.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://api.brevo.com/v3/smtp/email" || r.Header.Get("api-key") != "test-key" {
					t.Fatal("wrong Brevo request")
				}
				var p map[string]any
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Fatal(err)
				}
				if p["textContent"] != "plain" || p["htmlContent"] != "<p>html</p>" || p["headers"].(map[string]any)["X-Mailin-custom"] != "attempt_id:attempt" {
					t.Fatal("missing content or correlation")
				}
				if tc.status == 0 {
					return nil, errors.New("transport failure")
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Retry-After": []string{"60"}}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			got := b.send(context.Background(), job{ID: "notification", AttemptID: "attempt", Recipient: "recipient@example.com", Number: 1}, content{Subject: "subject", HTML: "<p>html</p>", Text: "plain"})
			if got.Status != tc.want {
				t.Fatalf("%+v", got)
			}
			if tc.status == 201 && tc.want == "accepted" && got.MessageID != "provider@example.com" {
				t.Fatal("message ID not normalized")
			}
			if tc.status == 429 && got.RetryAfter != time.Minute {
				t.Fatal("Retry-After ignored")
			}
		})
	}
}
func TestWebhookRejectsBeforeDatabaseAccess(t *testing.T) {
	token := strings.Repeat("s", 32)
	for _, tc := range []struct {
		auth, body string
		want       int
	}{
		{"", `{}`, 401}, {"Bearer wrong", `{}`, 401}, {"Bearer " + token, `{`, 400}, {"Bearer " + token, `{} {}`, 400}, {"Bearer " + token, `{"event":"delivered","email":"bad"}`, 400},
		{"Bearer " + token, `{"event":"opened","email":"user@example.com","message-id":"abc","ts_event":1}`, 204},
		{"Bearer " + token, strings.Repeat("x", 70<<10), 400},
	} {
		r := httptest.NewRequest(http.MethodPost, "/webhooks/brevo", strings.NewReader(tc.body))
		r.Header.Set("Authorization", tc.auth)
		w := httptest.NewRecorder()
		WebhookHandler(nil, token).ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d", w.Code, tc.want)
		}
	}
}
func TestNotificationConfig(t *testing.T) {
	for _, k := range []string{"NOTIFICATION_ENABLED", "BREVO_API_KEY", "NOTIFICATION_SENDER_EMAIL", "NOTIFICATION_SENDER_NAME", "BREVO_WEBHOOK_TOKEN"} {
		t.Setenv(k, "")
	}
	if c, e := LoadConfig(); e != nil || c.Enabled {
		t.Fatal("disabled defaults invalid")
	}
	t.Setenv("NOTIFICATION_ENABLED", "true")
	if _, e := LoadConfig(); e == nil {
		t.Fatal("missing secrets accepted")
	}
	t.Setenv("BREVO_API_KEY", "key")
	t.Setenv("NOTIFICATION_SENDER_EMAIL", "sender@example.com")
	t.Setenv("NOTIFICATION_SENDER_NAME", "OshePayment")
	t.Setenv("BREVO_WEBHOOK_TOKEN", strings.Repeat("x", 32))
	if _, e := LoadConfig(); e != nil {
		t.Fatal(e)
	}
}
