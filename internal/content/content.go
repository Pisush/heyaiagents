// Package content loads the seeded conference knowledgebase (agenda + speakers)
// into memory at startup and serves it read-only. Slides and transcripts are
// modelled now as nullable fields so they can be dropped in later without a
// schema change.
package content

// Session is one agenda entry. SlidesText and TranscriptText are nil until the
// "Later" content drop adds them; the shape is fixed now so that change is
// trivial.
type Session struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Track          string  `json:"track"`
	Time           string  `json:"time"`
	Abstract       string  `json:"abstract"`
	SlidesText     *string `json:"slides_text,omitempty"`
	TranscriptText *string `json:"transcript_text,omitempty"`
}

// Speaker is a conference speaker tied to a single talk.
type Speaker struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Bio           string `json:"bio"`
	TalkSessionID string `json:"talk_session_id"`
}

// Store is the read-only in-memory knowledgebase. The seed loader (Milestone 2)
// populates it; the MCP server (Milestone 3) reads from it.
type Store struct {
	sessions    []Session
	speakers    []Speaker
	sessionByID map[string]Session
}

// New builds a Store from the given sessions and speakers, indexing sessions by
// id for O(1) lookup.
func New(sessions []Session, speakers []Speaker) *Store {
	idx := make(map[string]Session, len(sessions))
	for _, s := range sessions {
		idx[s.ID] = s
	}
	return &Store{sessions: sessions, speakers: speakers, sessionByID: idx}
}

// Sessions returns all agenda sessions.
func (s *Store) Sessions() []Session { return s.sessions }

// Speakers returns all speakers.
func (s *Store) Speakers() []Speaker { return s.speakers }

// Session looks up a single session by id.
func (s *Store) Session(id string) (Session, bool) {
	sess, ok := s.sessionByID[id]
	return sess, ok
}

// Count reports the number of seeded sessions (used for the Completionist
// achievement threshold).
func (s *Store) Count() int { return len(s.sessions) }
