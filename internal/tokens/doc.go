// Package tokens signs and verifies proof-of-fetch tokens. A token is an
// HMAC-SHA256 over {session_id, issued_at, nonce} keyed by SERVER_SECRET, so
// verification is stateless and the server stays read-only.
//
// Implemented in Milestone 4.
package tokens
