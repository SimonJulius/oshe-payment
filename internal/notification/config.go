package notification

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Enabled                                       bool
	TestAPIEnabled                                bool
	TestAPIToken                                  string
	APIKey, SenderEmail, SenderName, WebhookToken string
}

func LoadConfig() (Config, error) {
	c := Config{APIKey: strings.TrimSpace(os.Getenv("BREVO_API_KEY")), SenderEmail: strings.TrimSpace(os.Getenv("NOTIFICATION_SENDER_EMAIL")), SenderName: strings.TrimSpace(os.Getenv("NOTIFICATION_SENDER_NAME")), WebhookToken: strings.TrimSpace(os.Getenv("BREVO_WEBHOOK_TOKEN"))}
	if v := os.Getenv("NOTIFICATION_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return c, errors.New("NOTIFICATION_ENABLED must be a boolean")
		}
		c.Enabled = b
	}
	if v := os.Getenv("NOTIFICATION_TEST_API_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return c, errors.New("NOTIFICATION_TEST_API_ENABLED must be a boolean")
		}
		c.TestAPIEnabled = b
	}
	c.TestAPIToken = strings.TrimSpace(os.Getenv("NOTIFICATION_TEST_API_TOKEN"))
	if c.TestAPIEnabled && (!c.Enabled || len(c.TestAPIToken) < 32 || strings.ContainsAny(c.TestAPIToken, "\r\n") || c.TestAPIToken == c.WebhookToken || c.TestAPIToken == c.APIKey) {
		return Config{}, errors.New("notification test API requires notifications enabled and a separate NOTIFICATION_TEST_API_TOKEN of at least 32 characters")
	}
	if !c.Enabled {
		return c, nil
	}
	if c.APIKey == "" || !validEmail(c.SenderEmail) || c.SenderName == "" || len(c.WebhookToken) < 32 || strings.ContainsAny(c.APIKey+c.WebhookToken, "\r\n") {
		return Config{}, errors.New("notifications require BREVO_API_KEY, a valid NOTIFICATION_SENDER_EMAIL, NOTIFICATION_SENDER_NAME and BREVO_WEBHOOK_TOKEN (at least 32 characters)")
	}
	return c, nil
}
