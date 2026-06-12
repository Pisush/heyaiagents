// Package challenges loads the sealed-core challenge bank: pre-authored
// questions whose answers gate data cores. The platform stays LLM-free; the
// "requires intelligence" property lives in the questions, and verification
// is a normalized string comparison.
package challenges

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"unicode"
)

// Challenge is one question with its accepted answers.
type Challenge struct {
	ID      string   `json:"id"`
	Q       string   `json:"q"`
	Answers []string `json:"answers"`
}

// Bank is the loaded challenge set.
type Bank struct {
	byID map[string]*Challenge
	ids  []string
}

// Load reads the bank from path; a missing file yields an empty bank
// (all cores spawn as plain speed cores).
func Load(path string) (*Bank, error) {
	b := &Bank{byID: map[string]*Challenge{}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return b, nil
	}
	if err != nil {
		return nil, fmt.Errorf("challenges: read %s: %w", path, err)
	}
	var list []*Challenge
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("challenges: parse %s: %w", path, err)
	}
	for _, c := range list {
		if c.ID == "" || c.Q == "" || len(c.Answers) == 0 {
			return nil, fmt.Errorf("challenges: entry missing id, q, or answers")
		}
		b.byID[c.ID] = c
		b.ids = append(b.ids, c.ID)
	}
	return b, nil
}

// Size reports how many challenges are loaded.
func (b *Bank) Size() int { return len(b.ids) }

// Random returns a random challenge, or nil when the bank is empty.
func (b *Bank) Random() *Challenge {
	if len(b.ids) == 0 {
		return nil
	}
	return b.byID[b.ids[rand.Intn(len(b.ids))]]
}

// Question returns the public question text for a challenge ID.
func (b *Bank) Question(id string) (string, bool) {
	c, ok := b.byID[id]
	if !ok {
		return "", false
	}
	return c.Q, true
}

// Verify reports whether answer matches any accepted answer for id,
// after normalization (case, punctuation, articles, whitespace).
func (b *Bank) Verify(id, answer string) bool {
	c, ok := b.byID[id]
	if !ok {
		return false
	}
	got := Normalize(answer)
	if got == "" {
		return false
	}
	for _, a := range c.Answers {
		if Normalize(a) == got {
			return true
		}
	}
	return false
}

// Normalize lowercases, strips everything but letters and digits, and drops
// leading articles, so "The Towel!" matches "towel".
func Normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, art := range []string{"the ", "a ", "an "} {
		s = strings.TrimPrefix(s, art)
	}
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
