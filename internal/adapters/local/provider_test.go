package local

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pax-beehive/memory-adaptor/internal/memory"
)

func TestProviderPutAndSearch(t *testing.T) {
	t.Parallel()

	provider, err := New("local", filepath.Join(t.TempDir(), "memory.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := provider.Put(context.Background(), memory.MemoryItem{
		Text:   "adapter registry fans out recall across enabled providers",
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Provider != "local" || ref.ID == "" {
		t.Fatalf("unexpected ref: %#v", ref)
	}

	hits, err := provider.Search(context.Background(), memory.SearchQuery{
		Text:  "enabled providers",
		Limit: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected one hit, got %d", len(hits))
	}
	if hits[0].Text == "" || hits[0].Score == 0 {
		t.Fatalf("unexpected hit: %#v", hits[0])
	}
}
