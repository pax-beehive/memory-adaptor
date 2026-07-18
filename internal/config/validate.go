package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func Validate(cfg Config) error {
	if err := validateProviderScoreSemantics(cfg.Providers); err != nil {
		return err
	}
	if err := validateMem0SearchScopePayloads(cfg.Providers); err != nil {
		return err
	}
	if err := validateIdentity(cfg.Identity, cfg.Agents); err != nil {
		return err
	}
	if err := validateCaptureQueue(cfg.CaptureQueue); err != nil {
		return err
	}
	if err := validateAgentOwners(cfg.Agents); err != nil {
		return err
	}
	if err := validateAgentRecallTimeouts(cfg.Agents); err != nil {
		return err
	}
	if err := validateRecallProfiles(cfg.RecallProfiles); err != nil {
		return err
	}
	return validateWriteProfiles(cfg.WriteProfiles)
}

func validateMem0SearchScopePayloads(providers map[string]ProviderConfig) error {
	for name, provider := range providers {
		providerType := strings.ToLower(strings.TrimSpace(provider.Type))
		if providerType == "" {
			providerType = strings.ToLower(strings.TrimSpace(name))
		}
		if providerType != "mem0" {
			continue
		}
		if _, err := ParseMem0SearchScopePayload(provider.SearchScopePayload); err != nil {
			return fmt.Errorf("provider %q: %w", name, err)
		}
	}
	return nil
}

func validateProviderScoreSemantics(providers map[string]ProviderConfig) error {
	for name, provider := range providers {
		providerType := strings.ToLower(strings.TrimSpace(provider.Type))
		if providerType == "" {
			providerType = strings.ToLower(strings.TrimSpace(name))
		}
		if providerType != "mem0" && providerType != "mem0-cloud" {
			continue
		}
		if _, err := ParseScoreSemantics(provider.ScoreSemantics); err != nil {
			return fmt.Errorf("provider %q: %w", name, err)
		}
	}
	return nil
}

func validateIdentity(identity IdentityConfig, agents map[string]AgentConfig) error {
	if strings.TrimSpace(identity.UserID) != "" && slugID(identity.UserID) == "" {
		return errors.New("identity.user_id must contain letters or numbers")
	}
	for name, agent := range agents {
		if strings.TrimSpace(agent.AgentID) != "" && slugID(agent.AgentID) == "" {
			return fmt.Errorf("agent %q agent_id must contain letters or numbers", name)
		}
	}
	return nil
}

func validateCaptureQueue(queue CaptureQueueConfig) error {
	for name, value := range map[string]string{
		"max_episode_age": queue.MaxEpisodeAge,
		"retry_min":       queue.RetryMin,
	} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return fmt.Errorf("capture_queue.%s must be a positive duration", name)
		}
	}
	for provider, concurrency := range queue.ProviderConcurrency {
		if concurrency <= 0 {
			return fmt.Errorf("capture_queue.provider_concurrency.%s must be positive", provider)
		}
	}
	if queue.MaxAttempts < 0 {
		return errors.New("capture_queue.max_attempts must not be negative")
	}
	return nil
}

func validateAgentOwners(agents map[string]AgentConfig) error {
	for name, agent := range agents {
		owner := strings.TrimSpace(strings.ToLower(agent.Integration.Owner))
		if owner == "" || owner == IntegrationOwnerPaxm || owner == IntegrationOwnerCodexPlugin || owner == IntegrationOwnerClaudePlugin {
			if owner == IntegrationOwnerCodexPlugin && name != "codex" {
				return fmt.Errorf("agent %q cannot use integration owner %q", name, owner)
			}
			if owner == IntegrationOwnerClaudePlugin && name != "claude" {
				return fmt.Errorf("agent %q cannot use integration owner %q", name, owner)
			}
			continue
		}
		return fmt.Errorf("agent %q has invalid integration owner %q; expected paxm, codex-plugin, or claude-plugin", name, agent.Integration.Owner)
	}
	return nil
}

func validateRecallProfiles(profiles map[string]RecallProfileConfig) error {
	recallNames := sortedKeys(profiles)
	for _, name := range recallNames {
		for _, route := range profiles[name].Providers {
			if err := validatePositiveDuration(route.Timeout); err != nil {
				return fmt.Errorf("recall profile %q provider %q timeout: %w", name, route.Name, err)
			}
		}
		for _, tier := range profiles[name].Tiers {
			if _, ok := canonicalTier(tier); !ok {
				return fmt.Errorf("recall profile %q has invalid tier %q; expected stm or ltm", name, tier)
			}
		}
	}
	return nil
}

func validateAgentRecallTimeouts(agents map[string]AgentConfig) error {
	for name, agent := range agents {
		for event, hook := range agent.Hooks {
			if err := validatePositiveDuration(hook.Recall.Timeout); err != nil {
				return fmt.Errorf("agent %q hook %q recall timeout: %w", name, event, err)
			}
			if err := validatePositiveDuration(hook.Recall.TimeoutExtra); err != nil {
				return fmt.Errorf("agent %q hook %q recall timeout_extra: %w", name, event, err)
			}
			if hook.Recall.Initial != nil {
				if err := validatePositiveDuration(hook.Recall.Initial.Timeout); err != nil {
					return fmt.Errorf("agent %q hook %q initial recall timeout: %w", name, event, err)
				}
				if err := validatePositiveDuration(hook.Recall.Initial.TimeoutExtra); err != nil {
					return fmt.Errorf("agent %q hook %q initial recall timeout_extra: %w", name, event, err)
				}
			}
		}
	}
	return nil
}

func validatePositiveDuration(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return errors.New("must be a positive duration")
	}
	return nil
}

func validateWriteProfiles(profiles map[string]WriteProfileConfig) error {
	writeNames := sortedKeys(profiles)
	if err := validateWriteProfileScopes(writeNames, profiles); err != nil {
		return err
	}
	for _, name := range writeNames {
		profile := profiles[name]
		for _, route := range profile.Providers {
			if err := validatePositiveDuration(route.Timeout); err != nil {
				return fmt.Errorf("write profile %q provider %q timeout: %w", name, route.Name, err)
			}
		}
		tier := strings.TrimSpace(profile.Tier)
		if tier != "" {
			var ok bool
			tier, ok = canonicalTier(tier)
			if !ok {
				return fmt.Errorf("write profile %q has invalid tier %q; expected stm or ltm", name, profile.Tier)
			}
		} else if strings.EqualFold(strings.TrimSpace(name), "stm") {
			tier = "stm"
		} else {
			tier = "ltm"
		}

		expiresAfter := strings.TrimSpace(profile.ExpiresAfter)
		if tier == "ltm" {
			if expiresAfter != "" {
				return fmt.Errorf("write profile %q with tier ltm must not set expires_after", name)
			}
			continue
		}
		if expiresAfter == "" {
			return fmt.Errorf("write profile %q with tier stm requires expires_after", name)
		}
		duration, err := time.ParseDuration(expiresAfter)
		if err != nil {
			return fmt.Errorf("write profile %q has invalid expires_after %q: %w", name, profile.ExpiresAfter, err)
		}
		if duration <= 0 {
			return fmt.Errorf("write profile %q expires_after must be positive", name)
		}
	}
	return nil
}

func validateWriteProfileScopes(names []string, profiles map[string]WriteProfileConfig) error {
	for _, name := range names {
		if err := validateMemoryScope(profiles[name].Scope); err != nil {
			return fmt.Errorf("write profile %q scope: %w", name, err)
		}
	}
	return nil
}

func validateMemoryScope(scope MemoryScopeConfig) error {
	typeName := strings.ToLower(strings.TrimSpace(scope.Type))
	if typeName == "" && strings.TrimSpace(scope.ID) == "" {
		return nil
	}
	if typeName != "personal" && typeName != "team" {
		return fmt.Errorf("invalid type %q; expected personal or team", scope.Type)
	}
	if slugID(scope.ID) == "" {
		return errors.New("id must contain letters or numbers")
	}
	return nil
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func canonicalTier(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "stm":
		return "stm", true
	case "ltm":
		return "ltm", true
	default:
		return "", false
	}
}
