package tokens_test

import (
	"testing"

	"github.com/pisush/heyaiagents/internal/tokens"
)

func TestSignAndVerify(t *testing.T) {
	tok := tokens.Sign("testsecret", "ses-001")
	if tok.SessionID != "ses-001" {
		t.Errorf("got session_id %q, want ses-001", tok.SessionID)
	}
	if tok.Nonce == "" {
		t.Error("nonce should not be empty")
	}
	if tok.Signature == "" {
		t.Error("signature should not be empty")
	}
	if !tokens.Verify("testsecret", tok) {
		t.Error("Verify returned false for a freshly signed token")
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	tok := tokens.Sign("testsecret", "ses-001")
	if tokens.Verify("wrongsecret", tok) {
		t.Error("Verify should return false for wrong secret")
	}
}

func TestVerifyTamperedSig(t *testing.T) {
	tok := tokens.Sign("testsecret", "ses-001")
	tok.Signature = "0000000000000000000000000000000000000000000000000000000000000000"
	if tokens.Verify("testsecret", tok) {
		t.Error("Verify should return false for tampered signature")
	}
}

func TestVerifyTamperedSessionID(t *testing.T) {
	tok := tokens.Sign("testsecret", "ses-001")
	tok.SessionID = "ses-002" // tamper
	if tokens.Verify("testsecret", tok) {
		t.Error("Verify should return false when session_id is tampered")
	}
}

func TestDifferentNonces(t *testing.T) {
	tok1 := tokens.Sign("secret", "ses-001")
	tok2 := tokens.Sign("secret", "ses-001")
	if tok1.Nonce == tok2.Nonce {
		t.Error("two tokens should have different nonces")
	}
	if tok1.Signature == tok2.Signature {
		t.Error("two tokens with different nonces should have different signatures")
	}
}
