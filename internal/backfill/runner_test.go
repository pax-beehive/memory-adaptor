package backfill

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pax-beehive/memory-adaptor/internal/config"
	"github.com/pax-beehive/memory-adaptor/internal/facade"
	"github.com/pax-beehive/memory-adaptor/internal/memory"
	"github.com/pax-beehive/memory-adaptor/internal/sessions"
)

type countingProvider struct {
	items []memory.MemoryItem
}

func (p *countingProvider) Name() string { return "target" }
func (p *countingProvider) Search(context.Context, memory.SearchQuery) ([]memory.MemoryHit, error) {
	return nil, nil
}
func (p *countingProvider) Put(_ context.Context, item memory.MemoryItem) (memory.MemoryRef, error) {
	p.items = append(p.items, item)
	return memory.MemoryRef{ID: item.ID}, nil
}
func (p *countingProvider) Health(context.Context) error { return nil }

func TestRunnerResumesWithoutUploadingSucceededTurnsAgain(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"type":"session_meta","timestamp":"2026-07-01T10:00:00Z","payload":{"id":"session","cwd":"/repo"}}
{"type":"event_msg","timestamp":"2026-07-01T10:01:00Z","payload":{"type":"user_message","message":"question"}}
{"type":"event_msg","timestamp":"2026-07-01T10:02:00Z","payload":{"type":"agent_message","phase":"final_answer","message":"answer"}}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := &countingProvider{}
	router, err := memory.NewRouter([]memory.ProviderBinding{{Provider: provider, Write: true}})
	if err != nil {
		t.Fatal(err)
	}
	service := facade.New(config.Config{Version: 1}, router)
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runner := Runner{Store: store, Service: service}
	options := RunOptions{
		Scope:    Scope("config", "codex", "target"),
		RunID:    "first",
		Agent:    "codex",
		Provider: "target",
		Files:    []sessions.File{{Path: sessionPath, Size: 100}},
		Cutoff:   time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	}

	first, err := runner.Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	options.RunID = "second"
	second, err := runner.Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.items) != 1 || first.Uploaded != 1 || second.Uploaded != 0 || second.Skipped != 1 {
		t.Fatalf("unexpected resume result: writes=%d first=%#v second=%#v", len(provider.items), first, second)
	}
}
