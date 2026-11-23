package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Sender interface {
	send(context.Context, job, content) outcome
}
type Brevo struct {
	apiKey      string
	senderEmail string
	senderName  string
	client      *http.Client
	endpoint    string
}

func NewBrevo(apiKey, email, name string) *Brevo {
	return &Brevo{apiKey: apiKey, senderEmail: email, senderName: name, client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: "https://api.brevo.com/v3/smtp/email"}
}
func (b *Brevo) send(ctx context.Context, j job, c content) outcome {
	payload := map[string]any{"sender": map[string]string{"email": b.senderEmail, "name": b.senderName}, "to": []map[string]string{{"email": j.Recipient}}, "subject": c.Subject, "htmlContent": c.HTML, "textContent": c.Text, "headers": map[string]string{"X-Mailin-custom": "attempt_id:" + j.AttemptID, "IdempotencyKey": j.ID}, "tags": []string{"oshe-notification"}}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint, bytes.NewReader(body))
	if err != nil {
		return outcome{Status: "failed", Code: "provider_request"}
	}
	req.Header.Set("api-key", b.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := b.client.Do(req)
	// Network errors and 5xx can occur after acceptance. Brevo's provider-side
	// deduplication is an additional defense, not an exactly-once guarantee.
	if err != nil {
		return outcome{Status: "unknown", Code: "provider_transport"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		var result struct {
			MessageID string `json:"messageId"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result) != nil || (normalizeMessageID(result.MessageID) == "" || len(result.MessageID) > 512) {
			return outcome{Status: "unknown", Code: "provider_response"}
		}
		return outcome{Status: "accepted", MessageID: normalizeMessageID(result.MessageID)}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		delay := time.Duration(1<<min(j.Number, 5)) * time.Second
		if n, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && n > 0 {
			delay = max(delay, time.Duration(min(n, 3600))*time.Second)
		} else if until, e := http.ParseTime(resp.Header.Get("Retry-After")); e == nil {
			delay = max(delay, min(time.Until(until), time.Hour))
		}
		return outcome{Status: "retry", Code: "provider_rate_limit", RetryAfter: delay}
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusRequestTimeout {
		return outcome{Status: "unknown", Code: "provider_ambiguous"}
	}
	// Never persist raw provider errors: they can contain email addresses or content.
	return outcome{Status: "failed", Code: "provider_rejected"}
}
func normalizeMessageID(id string) string { return strings.Trim(strings.TrimSpace(id), "<>") }
