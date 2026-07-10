package sessions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadCodexTurns(t *testing.T) {
	path := writeSessionFile(t, `{"type":"session_meta","timestamp":"2026-07-01T10:00:00Z","payload":{"id":"codex-session","cwd":"/repo"}}
{"type":"event_msg","timestamp":"2026-07-01T10:01:00Z","payload":{"type":"user_message","message":"find the bug"}}
{"type":"event_msg","timestamp":"2026-07-01T10:01:10Z","payload":{"type":"agent_message","phase":"commentary","message":"checking"}}
{"type":"event_msg","timestamp":"2026-07-01T10:02:00Z","payload":{"type":"agent_message","phase":"final_answer","message":"fixed it"}}
{"type":"response_item","timestamp":"2026-07-01T10:02:00Z","payload":{"type":"message","role":"assistant","content":"duplicate"}}
`)

	turns, err := ReadFile("codex", path, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	assertTurn(t, turns, "codex-session", "/repo", "find the bug", "fixed it")
}

func TestReadClaudeTurnsExcludesSidechainsAndToolResults(t *testing.T) {
	path := writeSessionFile(t, `{"type":"user","uuid":"u1","sessionId":"claude-session","cwd":"/repo","timestamp":"2026-07-01T10:01:00Z","message":{"role":"user","content":"implement it"}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","sessionId":"claude-session","cwd":"/repo","timestamp":"2026-07-01T10:02:00Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"secret"},{"type":"text","text":"done"},{"type":"tool_use","name":"Read"}]}}
{"type":"user","uuid":"tool","parentUuid":"a1","sessionId":"claude-session","cwd":"/repo","timestamp":"2026-07-01T10:02:10Z","message":{"role":"user","content":[{"type":"tool_result","content":"large output"}]}}
{"type":"assistant","uuid":"side","parentUuid":"u1","sessionId":"claude-session","cwd":"/repo","isSidechain":true,"timestamp":"2026-07-01T10:03:00Z","message":{"role":"assistant","content":[{"type":"text","text":"ignore me"}]}}
`)

	turns, err := ReadFile("claude", path, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	assertTurn(t, turns, "claude-session", "/repo", "implement it", "done")
}

func TestReadPiTurnsAndCutoff(t *testing.T) {
	path := writeSessionFile(t, `{"type":"session","id":"pi-session","cwd":"/repo","timestamp":"2026-07-01T10:00:00Z"}
{"type":"message","id":"u1","parentId":null,"timestamp":"2026-07-01T10:01:00Z","message":{"role":"user","content":"first"}}
{"type":"message","id":"a1","parentId":"u1","timestamp":"2026-07-01T10:02:00Z","message":{"role":"assistant","content":[{"type":"text","text":"answer one"},{"type":"thinking","thinking":"hidden"}]}}
{"type":"message","id":"u2","parentId":"a1","timestamp":"2026-07-02T10:01:00Z","message":{"role":"user","content":"too late"}}
{"type":"message","id":"a2","parentId":"u2","timestamp":"2026-07-02T10:02:00Z","message":{"role":"assistant","content":[{"type":"text","text":"answer two"}]}}
`)

	cutoff := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	turns, err := ReadFile("pi", path, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	assertTurn(t, turns, "pi-session", "/repo", "first", "answer one")
}

func assertTurn(t *testing.T, turns []Turn, sessionID, workspace, user, assistant string) {
	t.Helper()
	if len(turns) != 1 {
		t.Fatalf("turn count = %d, want 1: %#v", len(turns), turns)
	}
	turn := turns[0]
	if turn.SessionID != sessionID || turn.Workspace != workspace || turn.User != user || turn.Assistant != assistant {
		t.Fatalf("unexpected turn: %#v", turn)
	}
	if turn.ID == "" || turn.CreatedAt.IsZero() {
		t.Fatalf("turn identity/time missing: %#v", turn)
	}
}

func writeSessionFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
