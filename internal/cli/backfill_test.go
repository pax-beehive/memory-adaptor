package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pax-beehive/memory-adaptor/internal/config"
)

func TestCLIBackfillForegroundReportsProgressAndResumes(t *testing.T) {
	configPath, sessionDir := writeBackfillFixture(t, true)
	t.Setenv("PAXM_CODEX_SESSIONS", sessionDir)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	args := []string{"--config", configPath, "backfill", "run", "--agent", "codex", "--provider", "archive", "--rate", "1000/s"}
	if code := Main(args, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("first backfill failed with code %d: %s", code, stderr.String())
	}
	for _, expected := range []string{"Backfill progress", "speed=", "ETA=", "Backfill complete: uploaded=1 skipped=0"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("foreground output missing %q: %s", expected, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := Main(args, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("resumed backfill failed with code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Backfill complete: uploaded=0 skipped=1") {
		t.Fatalf("resumed run uploaded the turn again: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := Main([]string{"--config", configPath, "backfill", "status", "--agent", "codex", "--provider", "archive"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("status failed with code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "state=completed") || !strings.Contains(stdout.String(), "skipped=1") {
		t.Fatalf("unexpected status: %s", stdout.String())
	}
}

func TestCLIBackfillRequiresExplicitCutoffForExistingAgentConfig(t *testing.T) {
	configPath, sessionDir := writeBackfillFixture(t, false)
	t.Setenv("PAXM_CODEX_SESSIONS", sessionDir)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"--config", configPath, "backfill", "scan", "--agent", "codex"}, nil, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "requires --before") {
		t.Fatalf("missing cutoff was not rejected: code=%d stderr=%s", code, stderr.String())
	}
}

func writeBackfillFixture(t *testing.T, withIntegrationTime bool) (string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	cfg := config.DefaultConfig(configPath)
	cfg.Providers = map[string]config.ProviderConfig{
		"archive": {Type: "sqlite", Enabled: true, Path: filepath.Join(dir, "archive.sqlite")},
	}
	agent := cfg.Agents["codex"]
	if withIntegrationTime {
		agent.PassiveWriteStartedAt = "2026-07-02T00:00:00Z"
	}
	cfg.Agents["codex"] = agent
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	sessionDir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"session_meta","timestamp":"2026-07-01T10:00:00Z","payload":{"id":"session","cwd":"/repo"}}
{"type":"event_msg","timestamp":"2026-07-01T10:01:00Z","payload":{"type":"user_message","message":"historical question"}}
{"type":"event_msg","timestamp":"2026-07-01T10:02:00Z","payload":{"type":"agent_message","phase":"final_answer","message":"historical answer"}}
`
	if err := os.WriteFile(filepath.Join(sessionDir, "session.jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath, sessionDir
}
