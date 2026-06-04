package seed

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/pisush/heyaiagents/internal/content"
)

//go:embed *.json
var files embed.FS

// Load reads sessions.json and speakers.json from the embedded filesystem and
// returns them as typed slices ready to pass to content.New.
func Load() ([]content.Session, []content.Speaker, error) {
	sessData, err := files.ReadFile("sessions.json")
	if err != nil {
		return nil, nil, fmt.Errorf("read sessions.json: %w", err)
	}
	var sessions []content.Session
	if err := json.Unmarshal(sessData, &sessions); err != nil {
		return nil, nil, fmt.Errorf("parse sessions.json: %w", err)
	}

	spkData, err := files.ReadFile("speakers.json")
	if err != nil {
		return nil, nil, fmt.Errorf("read speakers.json: %w", err)
	}
	var speakers []content.Speaker
	if err := json.Unmarshal(spkData, &speakers); err != nil {
		return nil, nil, fmt.Errorf("parse speakers.json: %w", err)
	}

	return sessions, speakers, nil
}
