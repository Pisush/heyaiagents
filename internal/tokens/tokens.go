// Package tokens provides HMAC-SHA256 proof-of-fetch token signing and
// verification. Tokens are stateless: the server signs them on get_session and
// verifies them on POST /claim without storing any state.
package tokens

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Token is a signed proof-of-fetch record. The agent banks one per session it
// fetches and presents them all on POST /claim.
type Token struct {
	SessionID string `json:"session_id"`
	IssuedAt  int64  `json:"issued_at"` // unix timestamp (seconds)
	Nonce     string `json:"nonce"`
	Signature string `json:"sig"`
}

// Sign creates a new Token for sessionID, signed with secret. It generates a
// fresh random nonce and stamps the current time.
func Sign(secret, sessionID string) Token {
	nonce := randomHex(16)
	issuedAt := time.Now().Unix()
	sig := sign(secret, sessionID, issuedAt, nonce)
	return Token{
		SessionID: sessionID,
		IssuedAt:  issuedAt,
		Nonce:     nonce,
		Signature: sig,
	}
}

// Verify reports whether t's signature is valid for the given secret. It does
// NOT check the event window — that is the caller's responsibility (config.WithinEventWindow).
func Verify(secret string, t Token) bool {
	expected := sign(secret, t.SessionID, t.IssuedAt, t.Nonce)
	return hmac.Equal([]byte(expected), []byte(t.Signature))
}

// sign computes HMAC-SHA256 over "{session_id}:{issued_at}:{nonce}".
func sign(secret, sessionID string, issuedAt int64, nonce string) string {
	msg := fmt.Sprintf("%s:%d:%s", sessionID, issuedAt, nonce)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// randomHex returns n random bytes encoded as lowercase hex.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is unrecoverable on any sane OS.
		panic(fmt.Sprintf("tokens: crypto/rand: %v", err))
	}
	return hex.EncodeToString(b)
}
