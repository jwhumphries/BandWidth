// Package mail sends transactional email.
package mail

import (
	"errors"
	"fmt"

	gomail "github.com/wneessen/go-mail"
)

// Mailer sends transactional email. When not configured it is disabled and
// dependent features (password reset) are hidden.
type Mailer interface {
	Enabled() bool
	Send(to, subject, body string) error
}

// Disabled is a Mailer that refuses to send.
type Disabled struct{}

// Enabled always reports false.
func (Disabled) Enabled() bool { return false }

// Send always fails.
func (Disabled) Send(string, string, string) error {
	return errors.New("mail is not configured")
}

// Config holds SMTP settings; empty Host or From disables mail.
type Config struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// New returns an SMTP-backed Mailer, or Disabled when unconfigured.
func New(cfg Config) Mailer {
	if cfg.Host == "" || cfg.From == "" {
		return Disabled{}
	}
	return &smtpMailer{cfg: cfg}
}

type smtpMailer struct{ cfg Config }

func (m *smtpMailer) Enabled() bool { return true }

func (m *smtpMailer) Send(to, subject, body string) error {
	msg := gomail.NewMsg()
	if err := msg.From(m.cfg.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextPlain, body)

	opts := []gomail.Option{
		gomail.WithPort(m.cfg.Port),
		// Mandatory TLS: failing to send beats sending credentials in the
		// clear if STARTTLS is unavailable (or stripped by a middlebox).
		gomail.WithTLSPortPolicy(gomail.TLSMandatory),
	}
	if m.cfg.User != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(m.cfg.User),
			gomail.WithPassword(m.cfg.Pass),
		)
	}
	client, err := gomail.NewClient(m.cfg.Host, opts...)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	return client.DialAndSend(msg)
}
