package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func validateCaptureQueue(cfg Config) error {
	for name, value := range map[string]string{"max_episode_age": cfg.CaptureQueue.MaxEpisodeAge, "retry_min": cfg.CaptureQueue.RetryMin} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return fmt.Errorf("capture_queue.%s must be a positive duration", name)
		}
	}
	for provider, concurrency := range cfg.CaptureQueue.ProviderConcurrency {
		if concurrency <= 0 {
			return fmt.Errorf("capture_queue.provider_concurrency.%s must be positive", provider)
		}
	}
	if cfg.CaptureQueue.MaxAttempts < 0 {
		return errors.New("capture_queue.max_attempts must not be negative")
	}
	return nil
}

func validateAgentIntegrationOwners(cfg Config) error {
	for name, agent := range cfg.Agents {
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

func validateRecallProfiles(cfg Config) error {
	for _, name := range sortedKeys(cfg.RecallProfiles) {
		for _, route := range cfg.RecallProfiles[name].Providers {
			if err := validatePositiveDuration(route.Timeout); err != nil {
				return fmt.Errorf("recall profile %q provider %q timeout: %w", name, route.Name, err)
			}
		}
		for _, tier := range cfg.RecallProfiles[name].Tiers {
			if _, ok := canonicalTier(tier); !ok {
				return fmt.Errorf("recall profile %q has invalid tier %q; expected stm or ltm", name, tier)
			}
		}
	}
	return nil
}

func validateAgentRecallTimeouts(cfg Config) error {
	for name, agent := range cfg.Agents {
		for event, hook := range agent.Hooks {
			if err := validatePositiveDuration(hook.Recall.Timeout); err != nil {
				return fmt.Errorf("agent %q hook %q recall timeout: %w", name, event, err)
			}
			if hook.Recall.Initial != nil {
				if err := validatePositiveDuration(hook.Recall.Initial.Timeout); err != nil {
					return fmt.Errorf("agent %q hook %q initial recall timeout: %w", name, event, err)
				}
			}
		}
	}
	return nil
}

func validateWriteProfiles(cfg Config) error {
	for _, name := range sortedKeys(cfg.WriteProfiles) {
		profile := cfg.WriteProfiles[name]
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
