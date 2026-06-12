// Package vendors loads the booth-vendor registry: who may grant ink, how
// much, against what budget, and which one-time voucher codes belong to whom.
// The registry file is read once at boot and never written; redemption state
// (used codes, spent budgets) lives in the board store.
package vendors

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Vendor is one booth that can grant ink to agents.
type Vendor struct {
	// ID is the stable identifier agents use with visit_booth.
	ID string `json:"id"`
	// Name is the public name shown on the big screen.
	Name string `json:"name"`
	// Key authorizes POST /vendor/grant (Authorization: Bearer <key>).
	Key string `json:"key"`
	// Pitch is the text visit_booth returns - the vendor's invitation,
	// editable in the registry file without a code change.
	Pitch string `json:"pitch"`
	// Grant is the default ink per voucher code or grant call.
	Grant int `json:"grant"`
	// Budget caps the vendor's total ink across codes and grants.
	Budget int `json:"budget"`
	// Codes are this vendor's one-time voucher codes.
	Codes []string `json:"codes"`
}

// Registry is the set of configured vendors.
type Registry struct {
	vendors []*Vendor
	byID    map[string]*Vendor
	byCode  map[string]*Vendor
	byKey   map[string]*Vendor
}

// Load reads the registry from path. A missing file yields an empty registry
// (the platform runs fine with no vendors configured).
func Load(path string) (*Registry, error) {
	r := &Registry{byID: map[string]*Vendor{}, byCode: map[string]*Vendor{}, byKey: map[string]*Vendor{}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vendors: read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &r.vendors); err != nil {
		return nil, fmt.Errorf("vendors: parse %s: %w", path, err)
	}
	for _, v := range r.vendors {
		if v.ID == "" || v.Name == "" {
			return nil, fmt.Errorf("vendors: entry missing id or name")
		}
		if v.Grant <= 0 {
			v.Grant = 200
		}
		r.byID[v.ID] = v
		if v.Key != "" {
			r.byKey[v.Key] = v
		}
		for _, c := range v.Codes {
			r.byCode[NormalizeCode(c)] = v
		}
	}
	return r, nil
}

// All returns the configured vendors in file order.
func (r *Registry) All() []*Vendor { return r.vendors }

// ByID looks a vendor up by its identifier.
func (r *Registry) ByID(id string) (*Vendor, bool) {
	v, ok := r.byID[strings.ToLower(strings.TrimSpace(id))]
	return v, ok
}

// ByCode resolves a voucher code to its vendor.
func (r *Registry) ByCode(code string) (*Vendor, bool) {
	v, ok := r.byCode[NormalizeCode(code)]
	return v, ok
}

// ByKey resolves a bearer key to its vendor.
func (r *Registry) ByKey(key string) (*Vendor, bool) {
	v, ok := r.byKey[strings.TrimSpace(key)]
	return v, ok
}

// NormalizeCode canonicalizes a voucher code for comparison: trimmed,
// uppercased, separators dropped.
func NormalizeCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	return strings.NewReplacer("-", "", " ", "", "_", "").Replace(code)
}
