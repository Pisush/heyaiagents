package web

import (
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/pisush/heyaiagents/internal/store"
	"github.com/pisush/heyaiagents/internal/tokens"
)

const (
	maxDisplayNameLen  = 64
	maxSocialHandleLen = 64
	minDistinctSessions = 5
)

// claimRequest is the JSON body for POST /claim.
type claimRequest struct {
	Tokens           []tokens.Token `json:"tokens"`
	LeaderboardOptIn bool           `json:"leaderboard_opt_in"`
	DisplayName      string         `json:"display_name"`
	SocialHandle     string         `json:"social_handle"`
}

// claimResponse is the JSON body returned on a successful claim.
type claimResponse struct {
	DistinctSessionCount int      `json:"distinct_session_count"`
	Achievements         []string `json:"achievements"`
}

// errResponse is a JSON error envelope.
type errResponse struct {
	Error string `json:"error"`
}

func (h *Handler) claim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errResponse{"method not allowed"})
		return
	}

	var req claimRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResponse{"malformed JSON: " + err.Error()})
		return
	}

	// Sanitize text fields.
	req.DisplayName = sanitize(req.DisplayName, maxDisplayNameLen)
	req.SocialHandle = sanitize(req.SocialHandle, maxSocialHandleLen)

	// Verify tokens and collect distinct valid session IDs.
	distinct := make(map[string]struct{})
	for _, tok := range req.Tokens {
		if !tokens.Verify(h.cfg.ServerSecret, tok) {
			continue // invalid signature — skip
		}
		issuedAt := time.Unix(tok.IssuedAt, 0).UTC()
		if !h.cfg.WithinEventWindow(issuedAt) {
			continue // token outside the event window — skip
		}
		distinct[tok.SessionID] = struct{}{}
	}

	if len(distinct) < minDistinctSessions {
		writeJSON(w, http.StatusBadRequest, errResponse{
			"need at least 5 distinct valid sessions; got " + itoa(len(distinct)),
		})
		return
	}

	// Compute achievements.
	achievements := []string{"wall_of_fame"}
	if len(distinct) == h.content.Count() {
		achievements = append(achievements, "completionist")
	}

	entry := store.Entry{
		DisplayName:          req.DisplayName,
		SocialHandle:         req.SocialHandle,
		DistinctSessionCount: len(distinct),
		LeaderboardOptIn:     req.LeaderboardOptIn,
		Achievements:         achievements,
	}
	if err := h.leaderboard.Upsert(entry); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResponse{"could not save entry"})
		return
	}

	writeJSON(w, http.StatusOK, claimResponse{
		DistinctSessionCount: len(distinct),
		Achievements:         achievements,
	})
}

// sanitize strips HTML and truncates to maxLen runes.
func sanitize(s string, maxLen int) string {
	s = html.EscapeString(s)
	// Truncate by rune count to avoid splitting multi-byte sequences.
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen])
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.Encode(v) //nolint: ignore encode error on response writer
}

func itoa(n int) string { return strconv.Itoa(n) }
