package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SeriesState is stored at data/state/<slug>.json.
type SeriesState struct {
	SeriesSlug string               `json:"seriesSlug"`
	UpdatedAt  time.Time            `json:"updatedAt"`
	Issues     map[string]*IssueState `json:"issues"` // key is issue number as string
}

type IssueState struct {
	Number      int             `json:"number"`
	States      map[string]bool `json:"states"`
	InboxAudio  string          `json:"inboxAudio,omitempty"`
	InboxEbook  string          `json:"inboxEbook,omitempty"`
	OutputAudio string          `json:"outputAudio,omitempty"`
	OutputEbook string          `json:"outputEbook,omitempty"`
}

type Manager struct {
	BaseDir string
}

func New(baseDir string) *Manager {
	return &Manager{BaseDir: baseDir}
}

func (m *Manager) path(slug string) string {
	return filepath.Join(m.BaseDir, slug+".json")
}

func (m *Manager) Load(slug string) (SeriesState, error) {
	data, err := os.ReadFile(m.path(slug))
	if os.IsNotExist(err) {
		return SeriesState{
			SeriesSlug: slug,
			Issues:     make(map[string]*IssueState),
		}, nil
	}
	if err != nil {
		return SeriesState{}, fmt.Errorf("reading state for %s: %w", slug, err)
	}
	var s SeriesState
	if err := json.Unmarshal(data, &s); err != nil {
		return SeriesState{}, fmt.Errorf("parsing state for %s: %w", slug, err)
	}
	if s.Issues == nil {
		s.Issues = make(map[string]*IssueState)
	}
	return s, nil
}

func (m *Manager) Save(s SeriesState) error {
	if err := os.MkdirAll(m.BaseDir, 0755); err != nil {
		return err
	}
	s.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		return err
	}
	tmp := m.path(s.SeriesSlug) + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path(s.SeriesSlug))
}

func issueKey(number int) string {
	return fmt.Sprintf("%d", number)
}

// EnsureIssue creates an IssueState entry if it doesn't exist yet.
func (m *Manager) EnsureIssue(slug string, number int, stateNames []string) error {
	s, err := m.Load(slug)
	if err != nil {
		return err
	}
	key := issueKey(number)
	if s.Issues[key] == nil {
		states := make(map[string]bool)
		for _, name := range stateNames {
			states[name] = false
		}
		s.Issues[key] = &IssueState{
			Number: number,
			States: states,
		}
		return m.Save(s)
	}
	// Ensure new state keys are present (for series config updates)
	for _, name := range stateNames {
		if _, ok := s.Issues[key].States[name]; !ok {
			s.Issues[key].States[name] = false
		}
	}
	return m.Save(s)
}

func (m *Manager) SetState(slug string, number int, key string, val bool) error {
	s, err := m.Load(slug)
	if err != nil {
		return err
	}
	ik := issueKey(number)
	if s.Issues[ik] == nil {
		s.Issues[ik] = &IssueState{Number: number, States: make(map[string]bool)}
	}
	s.Issues[ik].States[key] = val
	return m.Save(s)
}

func (m *Manager) UpdateInbox(slug string, number int, mediaType string, path string) error {
	s, err := m.Load(slug)
	if err != nil {
		return err
	}
	ik := issueKey(number)
	if s.Issues[ik] == nil {
		s.Issues[ik] = &IssueState{Number: number, States: make(map[string]bool)}
	}
	switch mediaType {
	case "audio":
		s.Issues[ik].InboxAudio = path
	case "ebook":
		s.Issues[ik].InboxEbook = path
	}
	return m.Save(s)
}

func (m *Manager) ClearInbox(slug string, number int, mediaType string) error {
	return m.UpdateInbox(slug, number, mediaType, "")
}

func (m *Manager) UpdateOutput(slug string, number int, mediaType string, path string) error {
	s, err := m.Load(slug)
	if err != nil {
		return err
	}
	ik := issueKey(number)
	if s.Issues[ik] == nil {
		s.Issues[ik] = &IssueState{Number: number, States: make(map[string]bool)}
	}
	switch mediaType {
	case "audio":
		s.Issues[ik].OutputAudio = path
	case "ebook":
		s.Issues[ik].OutputEbook = path
	}
	return m.Save(s)
}
