package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestTOTPGenerateAndValidate(t *testing.T) {
	key, err := NewTOTPKey("alice")
	if err != nil {
		t.Fatalf("NewTOTPKey: %v", err)
	}
	if key.Secret == "" {
		t.Fatal("empty secret")
	}
	if !strings.Contains(key.URL, "BandWidth") || !strings.Contains(key.URL, "alice") {
		t.Errorf("URL %q missing issuer or account", key.URL)
	}

	code, err := totp.GenerateCode(key.Secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if !ValidateTOTP(code, key.Secret) {
		t.Error("current code rejected")
	}
	if ValidateTOTP("000000", key.Secret) {
		t.Error("wrong code accepted") // 1-in-a-million flake; rerun if it ever fires
	}
}
