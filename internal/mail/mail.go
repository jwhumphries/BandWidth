// Package mail sends transactional email.
package mail

import "fmt"

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
	return fmt.Errorf("mail is not configured")
}
