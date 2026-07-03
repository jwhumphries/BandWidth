package handlers

import "testing"

func TestIsAdminEmail(t *testing.T) {
	api := &API{AdminEmails: map[string]bool{"admin@example.com": true}}
	tests := []struct {
		email string
		want  bool
	}{
		{"admin@example.com", true},
		{"ADMIN@EXAMPLE.COM", true},
		{"  admin@example.com  ", true},
		{"nobody@example.com", false},
	}
	for _, tt := range tests {
		if got := api.IsAdminEmail(tt.email); got != tt.want {
			t.Errorf("IsAdminEmail(%q) = %v, want %v", tt.email, got, tt.want)
		}
	}
}

func TestIsAdminEmailNilSet(t *testing.T) {
	api := &API{}
	if api.IsAdminEmail("anyone@example.com") {
		t.Error("nil AdminEmails should match nobody")
	}
}
