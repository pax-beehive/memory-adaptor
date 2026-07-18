package config

import (
	"path/filepath"
	"strings"
)

func legacyJSONPath(path string) string {
	if !strings.EqualFold(filepath.Base(path), "config.yaml") {
		return path
	}
	return filepath.Join(filepath.Dir(path), "config.json")
}

func sqlitePathFromLegacyLocalPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return path
	}
	if strings.EqualFold(filepath.Ext(path), ".jsonl") {
		return strings.TrimSuffix(path, filepath.Ext(path)) + ".sqlite"
	}
	return path
}

func legacyRecallProfile(providers map[string]ProviderConfig) RecallProfileConfig {
	routes := make([]ProviderRouteConfig, 0, len(providers))
	for name, provider := range providers {
		if !provider.Enabled || !legacyBool(provider.Read, true) {
			continue
		}
		routes = append(routes, ProviderRouteConfig{
			Name:     name,
			Required: legacyBool(provider.Required, true),
			Weight:   legacyWeight(provider.Weight),
		})
	}
	return normalizeRecallProfile(RecallProfileConfig{
		Providers:  routes,
		MaxResults: defaultRecallMaxResults,
		Thresholds: RecallThresholdConfig{
			MinRelevance: defaultRecallMinRelevance,
			MinScore:     defaultRecallMinScore,
		},
		Ranking: RankingConfig{Type: "weighted_relevance"},
	})
}

func legacyWriteProfile(providers map[string]ProviderConfig) WriteProfileConfig {
	routes := make([]ProviderRouteConfig, 0, len(providers))
	for name, provider := range providers {
		if !provider.Enabled || !legacyBool(provider.Write, true) {
			continue
		}
		routes = append(routes, ProviderRouteConfig{
			Name:     name,
			Required: legacyBool(provider.Required, true),
			Weight:   legacyWeight(provider.Weight),
		})
	}
	return normalizeWriteProfile("default", WriteProfileConfig{Providers: routes})
}

func legacyAgents(hooks map[string]LegacyHookConfig) map[string]AgentConfig {
	if len(hooks) == 0 {
		return DefaultConfig("").Agents
	}
	agents := make(map[string]AgentConfig, len(hooks))
	for name, hook := range hooks {
		agent := AgentConfig{
			Enabled: hook.Enabled,
			ActiveRecall: ActiveRecallConfig{
				Enabled: true,
				Profile: "default",
				Output:  "markdown",
			},
			Hooks: make(map[string]AgentHookConfig),
		}
		for eventName, event := range hook.Events {
			recall := event.Recall
			if recall.Profile == "" {
				recall.Profile = "default"
			}
			agent.Hooks[eventName] = AgentHookConfig{Recall: recall}
		}
		agents[name] = normalizeAgent(agent)
	}
	return agents
}

func legacyBool(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func legacyWeight(weight float64) float64 {
	if weight == 0 {
		return defaultProviderRouteWeight
	}
	return weight
}
