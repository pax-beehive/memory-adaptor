package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	hookSessionStateVersion    = 2
	hookSessionStateMaxEntries = 1000
	hookSessionStateTTL        = 7 * 24 * time.Hour
	localTimeRefreshInterval   = 12 * time.Hour
)

// SessionState owns bounded hook-session policy state behind the capture seam.
// Storage failures are returned to the caller so hook adapters can fail open.
type SessionState struct{ path string }

type hookSessionState struct {
	Version  int                  `json:"version"`
	Seen     map[string]time.Time `json:"seen"`
	Activity map[string]time.Time `json:"activity,omitempty"`
}

func NewSessionState(path string) *SessionState { return &SessionState{path: path} }

func (s *SessionState) MarkInitial(event Event, now time.Time) (Event, error) {
	key := HookSessionStateKey(event)
	if key == "" {
		return event, nil
	}
	state, err := loadHookSessionState(s.path)
	if err != nil {
		return event, err
	}
	pruneHookSessionState(&state, now)
	_, exists := state.Seen[key]
	state.Seen[key] = now.UTC()
	if err := saveHookSessionState(s.path, state); err != nil {
		return event, err
	}
	if !exists {
		if event.Metadata == nil {
			event.Metadata = make(map[string]string)
		}
		event.Metadata[RecallPhaseMetadataKey] = RecallPhaseInitial
	}
	return event, nil
}

func (s *SessionState) Observe(event Event, now time.Time) (bool, error) {
	if !tracksSessionActivity(event.Event) {
		return false, nil
	}
	key := HookSessionStateKey(event)
	if key == "" {
		return false, nil
	}
	state, err := loadHookSessionState(s.path)
	if err != nil {
		return false, err
	}
	previous, exists := state.Activity[key]
	refresh := event.Event == "user_input" && exists && now.Sub(previous) > localTimeRefreshInterval
	state.Activity[key] = now.UTC()
	pruneHookSessionState(&state, now)
	if err := saveHookSessionState(s.path, state); err != nil {
		return false, err
	}
	return refresh, nil
}

func HookSessionStateKey(event Event) string {
	target := strings.TrimSpace(event.Target)
	if target == "" {
		target = "codex"
	}
	if value := strings.TrimSpace(event.Metadata["session_id"]); value != "" {
		return target + "/session/" + value
	}
	if value := strings.TrimSpace(event.Metadata["transcript_path"]); value != "" {
		return target + "/transcript/" + value
	}
	if value := strings.TrimSpace(event.Workspace); value != "" {
		return target + "/workspace/" + value
	}
	if value := strings.TrimSpace(event.Metadata["cwd"]); value != "" {
		return target + "/workspace/" + value
	}
	return ""
}

func tracksSessionActivity(event string) bool {
	switch event {
	case "session_start", "user_input", "turn_end":
		return true
	default:
		return false
	}
}

func loadHookSessionState(path string) (hookSessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newHookSessionState(), nil
		}
		return hookSessionState{}, err
	}
	var state hookSessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return newHookSessionState(), nil
	}
	if state.Version == 0 {
		state.Version = hookSessionStateVersion
	}
	if state.Seen == nil {
		state.Seen = make(map[string]time.Time)
	}
	if state.Activity == nil {
		state.Activity = make(map[string]time.Time)
	}
	return state, nil
}

func newHookSessionState() hookSessionState {
	return hookSessionState{Version: hookSessionStateVersion, Seen: make(map[string]time.Time), Activity: make(map[string]time.Time)}
}

func saveHookSessionState(path string, state hookSessionState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	state.Version = hookSessionStateVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func pruneHookSessionState(state *hookSessionState, now time.Time) {
	cutoff := now.Add(-hookSessionStateTTL)
	pruneHookTimes(state.Seen, cutoff)
	pruneHookTimes(state.Activity, cutoff)
}

func pruneHookTimes(values map[string]time.Time, cutoff time.Time) {
	for key, seenAt := range values {
		if seenAt.Before(cutoff) {
			delete(values, key)
		}
	}
	if len(values) <= hookSessionStateMaxEntries {
		return
	}
	type seenEntry struct {
		Key    string
		SeenAt time.Time
	}
	entries := make([]seenEntry, 0, len(values))
	for key, seenAt := range values {
		entries = append(entries, seenEntry{Key: key, SeenAt: seenAt})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].SeenAt.Before(entries[j].SeenAt) })
	for len(entries) > hookSessionStateMaxEntries {
		delete(values, entries[0].Key)
		entries = entries[1:]
	}
}
