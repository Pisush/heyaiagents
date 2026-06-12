package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

// adminRemoveAgent is the moderation endpoint: erase an agent's pixels and
// (by default) ban the ID so it cannot act again. A burned registration code
// stays burned, so on registration-code days a removed griefer cannot simply
// re-register.
//
//	POST /admin/remove_agent
//	Authorization: Bearer <ADMIN_KEY>
//	{"agent": "<agent_id or unique display name>", "ban": true}
func (h *Handler) adminRemoveAgent(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if h.cfg.AdminKey == "" || key != h.cfg.AdminKey {
		http.Error(w, `{"error":"invalid admin key"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		Agent string `json:"agent"`
		Ban   *bool  `json:"ban"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || req.Agent == "" {
		http.Error(w, `{"error":"body must be {\"agent\": \"<id or name>\", \"ban\": true|false}"}`, http.StatusBadRequest)
		return
	}
	ban := req.Ban == nil || *req.Ban // ban by default
	cleared, err := h.board.RemoveAgent(req.Agent, ban)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"removed": req.Agent, "banned": ban, "pixels_cleared": cleared,
	})
}
