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
	Query   string            `json:"query"`
	Profile string            `json:"profile,omitempty"`
	Limit   int               `json:"limit,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

type RecallResult struct {
	Query          string                 `json:"query"`
	Hits           []memory.MemoryHit     `json:"hits"`
	ProviderErrors []memory.ProviderError `json:"provider_errors,omitempty"`
}

type IngestInput struct {
	Text     string            `json:"text"`
	Profile  string            `json:"profile,omitempty"`
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
	policy, err := s.searchPolicy(input.Profile, input.Limit)
	if err != nil {
		return RecallResult{}, err
	}
	searchResult, err := s.router.SearchWithPolicy(ctx, memory.SearchQuery{
		Text:     query,
		Metadata: input.Meta,
	}, policy)
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
	policy, err := s.putPolicy(input.Profile)
	if err != nil {
		return IngestResult{}, err
	}
	putResult, err := s.router.PutWithPolicy(ctx, memory.MemoryItem{
		Text:      text,
		Source:    input.Source,
		Metadata:  input.Metadata,
		CreatedAt: time.Now().UTC(),
	}, policy)
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

	agentCfg, ok := s.cfg.Agents[event.Target]
	if !ok || !agentCfg.Enabled {
		result.Skipped = true
		return result, nil
	}
	eventCfg, ok := agentCfg.Hooks[event.Event]
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
		Query:   query,
		Profile: eventCfg.Recall.Profile,
		Limit:   limit,
		Meta:    event.Metadata,
	})
	result.Query = recall.Query
	result.Recall = &recall
	return result, err
}

func (s *Service) searchPolicy(profileName string, limitOverride int) (memory.SearchPolicy, error) {
	if strings.TrimSpace(profileName) == "" {
		profileName = s.defaultActiveRecallProfile()
	}
	profile, ok := s.cfg.RecallProfiles[profileName]
	if !ok {
		return memory.SearchPolicy{}, fmtMissingProfile("recall", profileName)
	}
	limit := profile.MaxResults
	if limitOverride > 0 {
		limit = limitOverride
	}
	return memory.SearchPolicy{
		Providers:    toMemoryRoutes(profile.Providers),
		Limit:        limit,
		MinRelevance: profile.Thresholds.MinRelevance,
		MinScore:     profile.Thresholds.MinScore,
		RecencyBoost: profile.Ranking.RecencyBoost,
	}, nil
}

func (s *Service) putPolicy(profileName string) (memory.PutPolicy, error) {
	if strings.TrimSpace(profileName) == "" {
		profileName = "default"
	}
	profile, ok := s.cfg.WriteProfiles[profileName]
	if !ok {
		return memory.PutPolicy{}, fmtMissingProfile("write", profileName)
	}
	return memory.PutPolicy{Providers: toMemoryRoutes(profile.Providers)}, nil
}

func (s *Service) defaultActiveRecallProfile() string {
	if agent, ok := s.cfg.Agents["codex"]; ok {
		if agent.ActiveRecall.Enabled && strings.TrimSpace(agent.ActiveRecall.Profile) != "" {
			return agent.ActiveRecall.Profile
		}
	}
	return "default"
}

func toMemoryRoutes(routes []config.ProviderRouteConfig) []memory.ProviderRoute {
	memoryRoutes := make([]memory.ProviderRoute, 0, len(routes))
	for _, route := range routes {
		memoryRoutes = append(memoryRoutes, memory.ProviderRoute{
			Name:     route.Name,
			Required: route.Required,
			Weight:   route.Weight,
		})
	}
	return memoryRoutes
}

func fmtMissingProfile(kind, name string) error {
	return errors.New(kind + " profile " + name + " is not configured")
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
