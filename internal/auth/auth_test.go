package auth

import (
	"testing"
	"time"
)

func TestPasswordHashVerify(t *testing.T) {
	hash, err := HashPassword("s3cret!")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "s3cret!") {
		t.Error("correct password rejected")
	}
	if VerifyPassword(hash, "wrong") {
		t.Error("wrong password accepted")
	}
}

func TestSessionRoundTrip(t *testing.T) {
	a := New([]byte("test-secret-key"))
	now := time.Now()
	tok := a.Issue("admin", now)
	sub, err := a.Verify(tok, now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if sub != "admin" {
		t.Errorf("subject = %q", sub)
	}
}

func TestSessionExpiry(t *testing.T) {
	a := New([]byte("k"))
	now := time.Now()
	tok := a.Issue("admin", now)
	future := now.Add(SessionTTL + time.Hour)
	if _, err := a.Verify(tok, future); err == nil {
		t.Error("expected expired token to fail")
	}
}

func TestSessionTamper(t *testing.T) {
	a := New([]byte("k"))
	other := New([]byte("different"))
	now := time.Now()
	tok := a.Issue("admin", now)
	// verifying with a different secret must fail
	if _, err := other.Verify(tok, now); err == nil {
		t.Error("token verified under wrong secret")
	}
	// mangled payload must fail
	if _, err := a.Verify(tok+"x", now); err == nil {
		t.Error("mangled token verified")
	}
}
