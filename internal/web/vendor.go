package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pisush/heyaiagents/internal/board"
)

// vendorGrant is the protected booth endpoint: a vendor (authenticated by its
// bearer key) grants ink to an agent by name or ID. This is the programmatic
// alternative to voucher codes - a vendor's own booth app or backend can call
// it directly.
//
//	POST /vendor/grant
//	Authorization: Bearer <vendor key>
//	{"agent": "<agent_id or unique display name>", "amount": 200, "note": "demo done"}
//
// amount is optional (defaults to the vendor's configured grant).
func (h *Handler) vendorGrant(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	v, ok := h.vendors.ByKey(key)
	if !ok || key == "" {
		http.Error(w, `{"error":"invalid vendor key"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Agent  string `json:"agent"`
		Amount int    `json:"amount"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}
	if req.Agent == "" {
		http.Error(w, `{"error":"agent is required (agent_id or unique display name)"}`, http.StatusBadRequest)
		return
	}
	amount := req.Amount
	if amount == 0 {
		amount = v.Grant
	}

	// Accept an agent_id directly, else resolve a unique display name.
	agent, found := h.board.Agent(req.Agent)
	if !found {
		var err error
		agent, err = h.board.FindByName(req.Agent)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, err.Error())
			return
		}
	}

	ink, err := h.board.GrantVendor(agent.ID, v.ID, v.Name, amount, v.Budget, "")
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"granted":     amount,
		"agent":       agent.Name,
		"agent_ink":   ink,
		"budget_left": v.Budget - h.board.VendorSpent(v.ID),
	})
}

// vendorSpawnCore lets a vendor drop a sponsored data core on demand:
//
//	POST /vendor/spawn_core
//	Authorization: Bearer <vendor key>
//
// Rate-limited per vendor; the bounty is debited from the vendor's budget.
func (h *Handler) vendorSpawnCore(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	v, ok := h.vendors.ByKey(key)
	if !ok || key == "" {
		http.Error(w, `{"error":"invalid vendor key"}`, http.StatusUnauthorized)
		return
	}
	core, err := h.board.SpawnVendorCore(v.ID, v.Name, board.CoreValue, v.Budget, 20*time.Minute)
	if err != nil {
		writeJSONError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"core":        core,
		"budget_left": v.Budget - h.board.VendorSpent(v.ID),
	})
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
