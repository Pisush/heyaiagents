// Package mcp serves the read-only Model Context Protocol surface over HTTP:
// list_sessions, get_session (which also issues a proof-of-fetch token),
// list_speakers, and get_leaderboard. It performs no writes.
//
// Implemented in Milestone 3 using github.com/modelcontextprotocol/go-sdk.
package mcp
