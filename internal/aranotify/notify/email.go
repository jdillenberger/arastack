package notify

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// EmailNotifier sends notifications via SMTP email.
type EmailNotifier struct {
	host     string
	port     int
	from     string
	to       []string
	username string
	password string
}

// NewEmailNotifier creates a new EmailNotifier.
func NewEmailNotifier(host string, port int, from string, to []string, username, password string) *EmailNotifier {
	return &EmailNotifier{
		host:     host,
		port:     port,
		from:     from,
		to:       to,
		username: username,
		password: password,
	}
}

func (e *EmailNotifier) Name() string { return "email" }

func (e *EmailNotifier) Send(_ context.Context, n Notification) error {
	severity := n.Severity
	if severity == "" {
		severity = "info"
	}

	subject := fmt.Sprintf("[%s] %s", severity, n.Title)
	body := n.Body

	msg := strings.Join([]string{
		"From: " + e.from,
		"To: " + strings.Join(e.to, ", "),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"utf-8\"",
		"",
		body,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	auth := smtp.PlainAuth("", e.username, e.password, e.host)

	if err := smtp.SendMail(addr, auth, e.from, e.to, []byte(msg)); err != nil {
		return fmt.Errorf("sending email: %w", err)
	}
	return nil
}
