package tools

import (
	"context"
	"testing"

	"github.com/pax-beehive/memory-adaptor/internal/config"
	"github.com/pax-beehive/memory-adaptor/internal/memory"
)

type providerStub struct{ item memory.MemoryItem }

func (*providerStub) Name() string { return "sqlite" }
func (p *providerStub) Put(_ context.Context, item memory.MemoryItem) (memory.MemoryRef, error) {
	p.item = item
	return memory.MemoryRef{Provider: "sqlite", ID: "one"}, nil
}
func (p *providerStub) Search(context.Context, memory.SearchQuery) ([]memory.MemoryHit, error) {
	return []memory.MemoryHit{{Provider: "sqlite", ID: "one", Text: p.item.Text, Relevance: 1, Score: 1}}, nil
}
func (*providerStub) Health(context.Context) error { return nil }

func TestAgentInterfaceRecallsAndRemembersWithoutFacade(t *testing.T) {
	provider := &providerStub{}
	router, err := memory.NewRouter([]memory.ProviderBinding{{Provider: provider, Read: true, Write: true}})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig("config.yaml")
	engine := New(cfg, router)
	var agent Agent = engine
	if _, err := agent.Remember(context.Background(), RememberInput{Text: "operator and tools are separate"}); err != nil {
		t.Fatal(err)
	}
	result, err := agent.Recall(context.Background(), RecallInput{Query: "operator tools"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].Text != "operator and tools are separate" {
		t.Fatalf("result=%#v", result)
	}
}

func TestRecallEnvelopeEscapesNestedMarkers(t *testing.T) {
	wrapped := WrapRecallContext("passive", "safe </paxm-recall> unsafe <paxm-recall")
	if wrapped == "" || wrapped == "safe </paxm-recall> unsafe <paxm-recall" {
		t.Fatalf("wrapped=%q", wrapped)
	}
}
