package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash = %q, want argon2id format", hash)
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Error("correct password rejected")
	}
	if VerifyPassword("wrong password", hash) {
		t.Error("wrong password accepted")
	}
	if VerifyPassword("anything", "not-a-hash") {
		t.Error("garbage hash accepted")
	}
}

func TestNewTokenIsRandomAndHashable(t *testing.T) {
	a, b := NewToken(), NewToken()
	if a == b {
		t.Fatal("two tokens are identical")
	}
	if len(a) < 40 {
		t.Fatalf("token too short: %d chars", len(a))
	}
	if HashToken(a) == HashToken(b) {
		t.Error("different tokens hash identically")
	}
	hash1 := HashToken(a)
	hash2 := HashToken(a)
	if hash1 != hash2 {
		t.Error("hash is not deterministic")
	}
}

func TestNewBackupCodes(t *testing.T) {
	codes := NewBackupCodes()
	if len(codes) != 10 {
		t.Fatalf("got %d codes, want 10", len(codes))
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if len(c) != 9 || c[4] != '-' {
			t.Errorf("code %q not in XXXX-XXXX format", c)
		}
		if seen[c] {
			t.Errorf("duplicate code %q", c)
		}
		seen[c] = true
	}
}
