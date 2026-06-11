// Package mail sends transactional email.
package mail

import "errors"

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
	return newSMTP(cfg)
}

// newSMTP is replaced with a real implementation in the mail task.
func newSMTP(cfg Config) Mailer {
	_ = cfg
	return Disabled{}
}
