package facade

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"text/template"
	"time"

	"github.com/pax-beehive/memory-adaptor/internal/config"
	"github.com/pax-beehive/memory-adaptor/internal/memory"
)

type Service struct {
	cfg    config.Config
	router *memory.Router
}

type RecallInput struct {
	Query string            `json:"query"`
	Limit int               `json:"limit,omitempty"`
	Meta  map[string]string `json:"meta,omitempty"`
}

type RecallResult struct {
	Query          string                 `json:"query"`
	Hits           []memory.MemoryHit     `json:"hits"`
	ProviderErrors []memory.ProviderError `json:"provider_errors,omitempty"`
}

type IngestInput struct {
	Text     string            `json:"text"`
	Source   string            `json:"source,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type IngestResult struct {
	Refs           []memory.MemoryRef     `json:"refs"`
	ProviderErrors []memory.ProviderError `json:"provider_errors,omitempty"`
}

type HookEvent struct {
	Target    string            `json:"target,omitempty"`
	Event     string            `json:"event,omitempty"`
	Query     string            `json:"query,omitempty"`
	Prompt    string            `json:"prompt,omitempty"`
	Workspace string            `json:"workspace,omitempty"`
	Limit     int               `json:"limit,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type HookResult struct {
	Target  string        `json:"target"`
	Event   string        `json:"event"`
	Query   string        `json:"query,omitempty"`
	Skipped bool          `json:"skipped,omitempty"`
	Recall  *RecallResult `json:"recall,omitempty"`
}

func New(cfg config.Config, router *memory.Router) *Service {
	return &Service{cfg: config.Normalize(cfg), router: router}
}

func (s *Service) Recall(ctx context.Context, input RecallInput) (RecallResult, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return RecallResult{}, errors.New("recall query is required")
	}
	searchResult, err := s.router.Search(ctx, memory.SearchQuery{
		Text:     query,
		Limit:    input.Limit,
		Metadata: input.Meta,
	})
	result := RecallResult{
		Query:          query,
		Hits:           searchResult.Hits,
		ProviderErrors: searchResult.ProviderErrors,
	}
	return result, err
}

func (s *Service) Ingest(ctx context.Context, input IngestInput) (IngestResult, error) {
	text := strings.TrimSpace(input.Text)
	if text == "" {
		return IngestResult{}, errors.New("ingest text is required")
	}
	putResult, err := s.router.Put(ctx, memory.MemoryItem{
		Text:      text,
		Source:    input.Source,
		Metadata:  input.Metadata,
		CreatedAt: time.Now().UTC(),
	})
	result := IngestResult{
		Refs:           putResult.Refs,
		ProviderErrors: putResult.ProviderErrors,
	}
	return result, err
}

func (s *Service) RunHook(ctx context.Context, event HookEvent) (HookResult, error) {
	if event.Target == "" {
		event.Target = "codex"
	}
	if event.Event == "" {
		event.Event = "user_prompt"
	}
	result := HookResult{Target: event.Target, Event: event.Event}

	hookCfg, ok := s.cfg.Hooks[event.Target]
	if !ok || !hookCfg.Enabled {
		result.Skipped = true
		return result, nil
	}
	eventCfg, ok := hookCfg.Events[event.Event]
	if !ok || !eventCfg.Recall.Enabled {
		result.Skipped = true
		return result, nil
	}

	query := strings.TrimSpace(event.Query)
	if query == "" {
		var err error
		query, err = renderHookQuery(eventCfg.Recall.QueryTemplate, event)
		if err != nil {
			return result, err
		}
	}
	if query == "" {
		query = event.Prompt
	}
	limit := event.Limit
	if limit == 0 {
		limit = eventCfg.Recall.MaxResults
	}
	recall, err := s.Recall(ctx, RecallInput{
		Query: query,
		Limit: limit,
		Meta:  event.Metadata,
	})
	result.Query = recall.Query
	result.Recall = &recall
	return result, err
}

func renderHookQuery(queryTemplate string, event HookEvent) (string, error) {
	if strings.TrimSpace(queryTemplate) == "" {
		return "", nil
	}
	tmpl, err := template.New("hook_query").Option("missingkey=zero").Parse(queryTemplate)
	if err != nil {
		return "", err
	}
	data := map[string]any{
		"target":    event.Target,
		"event":     event.Event,
		"query":     event.Query,
		"prompt":    event.Prompt,
		"workspace": event.Workspace,
		"metadata":  event.Metadata,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
