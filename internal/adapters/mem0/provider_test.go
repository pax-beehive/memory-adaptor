package mem0

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pax-beehive/memory-adaptor/internal/config"
	"github.com/pax-beehive/memory-adaptor/internal/memory"
)

func TestNewValidatesMem0Config(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]config.ProviderConfig{
		"missing target": {BaseURL: "http://localhost:8888"},
		"bad base url":   {BaseURL: "localhost:8888", UserID: "user-1"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := newWithClient("mem0", tc, http.DefaultClient); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestPutCreatesMem0Memory(t *testing.T) {
	t.Parallel()

	var captured addRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memories" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "mem0-key" {
			t.Fatalf("unexpected api key header: %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"mem-1","memory":"Todd prefers YAML config","event":"ADD"}]}`))
	}))
	defer server.Close()

	infer := false
	provider, err := New("mem0", config.ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "mem0-key",
		UserID:  "user-1",
		Infer:   &infer,
	})
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC)
	ref, err := provider.Put(context.Background(), memory.MemoryItem{
		ID:        "memory-1",
		Text:      "Todd prefers YAML config",
		Source:    "test",
		Metadata:  map[string]string{"project": "paxm"},
		CreatedAt: createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Provider != "mem0" || ref.ID != "mem-1" {
		t.Fatalf("unexpected ref: %#v", ref)
	}
	if captured.UserID != "user-1" || captured.AgentID != "" || captured.RunID != "" {
		t.Fatalf("unexpected target: %#v", captured)
	}
	if len(captured.Messages) != 1 || captured.Messages[0].Role != "user" || captured.Messages[0].Content != "Todd prefers YAML config" {
		t.Fatalf("unexpected messages: %#v", captured.Messages)
	}
	if captured.Infer == nil || *captured.Infer {
		t.Fatalf("infer flag was not forwarded: %#v", captured.Infer)
	}
	if captured.Metadata["paxm_id"] != "memory-1" || captured.Metadata["paxm_source"] != "test" || captured.Metadata["project"] != "paxm" {
		t.Fatalf("metadata was not mapped: %#v", captured.Metadata)
	}
}

func TestSearchMapsMem0Results(t *testing.T) {
	t.Parallel()

	var captured searchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"id": "mem-1",
					"memory": "YAML config is the paxm default",
					"score": 0.82,
					"user_id": "user-1",
					"metadata": {"project": "paxm"},
					"created_at": "2026-07-09T01:02:03Z",
					"score_details": {"semantic": 0.82}
				}
			]
		}`))
	}))
	defer server.Close()

	provider, err := New("mem0", config.ProviderConfig{
		BaseURL: server.URL,
		UserID:  "user-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := provider.Search(context.Background(), memory.SearchQuery{
		Text:     "paxm config",
		Limit:    200,
		Metadata: map[string]string{"project": "paxm", "user_id": "ignored"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.Query != "paxm config" || captured.TopK == nil || *captured.TopK != 100 {
		t.Fatalf("unexpected search request: %#v", captured)
	}
	if captured.Filters["user_id"] != "user-1" || captured.Filters["project"] != "paxm" {
		t.Fatalf("unexpected filters: %#v", captured.Filters)
	}
	if len(hits) != 1 {
		t.Fatalf("expected one hit, got %#v", hits)
	}
	hit := hits[0]
	if hit.ID != "mem-1" || hit.Text != "YAML config is the paxm default" || hit.Relevance != 0.82 {
		t.Fatalf("unexpected hit: %#v", hit)
	}
	if hit.RawScore == nil || *hit.RawScore != 0.82 || hit.RawScoreKind != "mem0_score" {
		t.Fatalf("unexpected score mapping: %#v", hit)
	}
	if hit.Metadata["project"] != "paxm" || hit.Metadata["mem0_user_id"] != "user-1" || hit.Metadata["mem0_score_details"] == "" {
		t.Fatalf("unexpected metadata: %#v", hit.Metadata)
	}
	if hit.CreatedAt.IsZero() {
		t.Fatalf("created_at was not parsed: %#v", hit)
	}
}

func TestHealthChecksOpenAPI(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi.json" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"openapi":"3.1.0"}`))
	}))
	defer server.Close()

	provider, err := New("mem0", config.ProviderConfig{BaseURL: server.URL, AgentID: "agent-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}
