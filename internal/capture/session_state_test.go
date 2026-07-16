package capture

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionStateInitialRecallAndLocalTimeRefresh(t *testing.T) {
	state := NewSessionState(filepath.Join(t.TempDir(), "session_state.json"))
	started := time.Date(2026, time.July, 16, 9, 0, 0, 0, time.UTC)
	input := func() Event {
		return Event{Target: "codex", Event: "user_input", Metadata: map[string]string{"session_id": "session-1"}}
	}

	first, err := state.MarkInitial(input(), started)
	if err != nil || first.Metadata[RecallPhaseMetadataKey] != RecallPhaseInitial {
		t.Fatalf("first input = %#v, err=%v", first, err)
	}
	second, err := state.MarkInitial(input(), started.Add(time.Minute))
	if err != nil || second.Metadata[RecallPhaseMetadataKey] != "" {
		t.Fatalf("second input = %#v, err=%v", second, err)
	}

	startEvent := Event{Target: "codex", Event: "session_start", Metadata: map[string]string{"session_id": "session-1"}}
	if refresh, err := state.Observe(startEvent, started); err != nil || refresh {
		t.Fatalf("session start refresh=%v err=%v", refresh, err)
	}
	if refresh, err := state.Observe(input(), started.Add(12*time.Hour)); err != nil || refresh {
		t.Fatalf("exactly twelve hours refresh=%v err=%v", refresh, err)
	}

	turnEnd := Event{Target: "codex", Event: "turn_end", Metadata: map[string]string{"session_id": "session-1"}}
	if refresh, err := state.Observe(turnEnd, started.Add(13*time.Hour)); err != nil || refresh {
		t.Fatalf("turn end refresh=%v err=%v", refresh, err)
	}
	if refresh, err := state.Observe(input(), started.Add(25*time.Hour)); err != nil || refresh {
		t.Fatalf("twelve hours after turn end refresh=%v err=%v", refresh, err)
	}
	if refresh, err := state.Observe(input(), started.Add(37*time.Hour+time.Second)); err != nil || !refresh {
		t.Fatalf("long turn gap refresh=%v err=%v", refresh, err)
	}
}

func TestSessionStateRefreshesBeforePruningLongIdleSession(t *testing.T) {
	state := NewSessionState(filepath.Join(t.TempDir(), "session_state.json"))
	started := time.Date(2026, time.July, 16, 9, 0, 0, 0, time.UTC)
	startEvent := Event{Target: "pi", Event: "session_start", Metadata: map[string]string{"session_id": "session-8d"}}
	input := Event{Target: "pi", Event: "user_input", Metadata: map[string]string{"session_id": "session-8d"}}
	if _, err := state.Observe(startEvent, started); err != nil {
		t.Fatal(err)
	}
	if refresh, err := state.Observe(input, started.Add(8*24*time.Hour)); err != nil || !refresh {
		t.Fatalf("eight-day refresh=%v err=%v", refresh, err)
	}
}

func TestSessionStateRefreshesWhenAnotherSessionPrunedItsActivity(t *testing.T) {
	state := NewSessionState(filepath.Join(t.TempDir(), "session_state.json"))
	started := time.Date(2026, time.July, 16, 9, 0, 0, 0, time.UTC)
	sessionStart := func(id string) Event {
		return Event{Target: "codex", Event: "session_start", Metadata: map[string]string{"session_id": id}}
	}
	if _, err := state.Observe(sessionStart("session-a"), started); err != nil {
		t.Fatal(err)
	}
	if _, err := state.Observe(sessionStart("session-b"), started.Add(8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	inputA := Event{Target: "codex", Event: "user_input", Metadata: map[string]string{"session_id": "session-a"}}
	if refresh, err := state.Observe(inputA, started.Add(8*24*time.Hour+time.Minute)); err != nil || !refresh {
		t.Fatalf("pruned session refresh=%v err=%v", refresh, err)
	}
}

func TestSessionStateMigratesInvalidAndLegacyStateFailOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session_state.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := NewSessionState(path)
	event := Event{Target: "opencode", Event: "user_input", Workspace: "/workspace"}
	marked, err := state.MarkInitial(event, time.Now())
	if err != nil || marked.Metadata[RecallPhaseMetadataKey] != RecallPhaseInitial {
		t.Fatalf("invalid state recovery = %#v, err=%v", marked, err)
	}
	if key := HookSessionStateKey(Event{Target: "claude", Metadata: map[string]string{"transcript_path": "/tmp/transcript"}}); key != "claude/transcript//tmp/transcript" {
		t.Fatalf("transcript key = %q", key)
	}
	if refresh, err := state.Observe(Event{Event: "tool_use"}, time.Now()); err != nil || refresh {
		t.Fatalf("untracked event refresh=%v err=%v", refresh, err)
	}
}
