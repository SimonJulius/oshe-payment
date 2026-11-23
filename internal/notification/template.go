package notification

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strings"
	texttemplate "text/template"
)

const InvitationTemplate = "invitation.v1"
const invitationHTML = `<!doctype html><html><body><p>Hello {{.name}},</p><p>{{.inviter}} has invited you to join {{.organization}} on OshePayment.</p><p><a href="{{.invitation_url}}">Accept invitation</a></p><p>This invitation expires at {{.expires_at}}.</p><p>If you were not expecting this invitation, you can ignore this email.</p></body></html>`
const invitationText = `Hello {{.name}},

{{.inviter}} has invited you to join {{.organization}} on OshePayment.

Accept invitation: {{.invitation_url}}

This invitation expires at {{.expires_at}}.
If you were not expecting this invitation, you can ignore this email.
`

var htmlInvitation = template.Must(template.New(InvitationTemplate).Option("missingkey=error").Parse(invitationHTML))
var textInvitation = texttemplate.Must(texttemplate.New(InvitationTemplate).Option("missingkey=error").Parse(invitationText))

type content struct{ Subject, HTML, Text string }

func render(key string, data map[string]string) (content, error) {
	if key != InvitationTemplate {
		return content{}, fmt.Errorf("%w: unsupported template", ErrInvalid)
	}
	if len(data) != 5 {
		return content{}, fmt.Errorf("%w: invitation requires name, inviter, organization, invitation_url, expires_at", ErrInvalid)
	}
	for _, k := range []string{"name", "inviter", "organization", "invitation_url", "expires_at"} {
		if strings.TrimSpace(data[k]) == "" || len(data[k]) > 4096 {
			return content{}, fmt.Errorf("%w: invalid template data", ErrInvalid)
		}
	}
	u, err := url.Parse(data["invitation_url"])
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return content{}, fmt.Errorf("%w: invitation URL must use HTTPS", ErrInvalid)
	}
	var h, t bytes.Buffer
	if err := htmlInvitation.Execute(&h, data); err != nil {
		return content{}, ErrInvalid
	}
	if err := textInvitation.Execute(&t, data); err != nil {
		return content{}, ErrInvalid
	}
	return content{Subject: "You’re invited to OshePayment", HTML: h.String(), Text: t.String()}, nil
}
