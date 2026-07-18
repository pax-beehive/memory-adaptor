package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var ErrConfigMissing = errors.New("paxm config is missing")

const (
	defaultConfigVersion       = 1
	defaultMem0BaseURL         = "http://localhost:8888"
	defaultMem0CloudBaseURL    = "https://api.mem0.ai"
	defaultMemOSBaseURL        = "http://localhost:8000"
	defaultMemOSCloudBaseURL   = "https://memos.memtensor.cn/api/openmem/v1"
	defaultOpenVikingBaseURL   = "http://localhost:1933"
	defaultJSONRPCTransport    = "stdio"
	defaultJSONRPCTimeout      = "30s"
	defaultProviderRouteWeight = 1
	defaultRecallMaxResults    = 3
	defaultRecallMinRelevance  = 0.25
	defaultRecallMinScore      = 0.25
	defaultSTMExpiresAfter     = "24h"

	passiveRecallMaxResults   = 2
	passiveRecallMinRelevance = 0.75
	passiveRecallMinScore     = 0.75

	initialRecallMaxResults   = 5
	initialRecallMinRelevance = 0.35
	initialRecallMinScore     = 0.35

	defaultHookRecallMaxResults      = passiveRecallMaxResults
	defaultPassiveRecallTimeout      = "800ms"
	defaultPassiveRecallTimeoutExtra = "100ms"
	defaultProviderRecallTimeout     = "250ms"
	defaultCloudRecallTimeout        = "800ms"
	defaultCloudRecallThreshold      = 0.20
	defaultProviderWriteTimeout      = "30s"
	defaultHookInsertionMinScore     = 0.8
	defaultHookInsertionMaxItems     = passiveRecallMaxResults
	defaultHookBufferFlushCount      = 10
	defaultTelemetryMaxEventFileSize = 1 << 20
	defaultTelemetryMaxEventFiles    = 3
	defaultTelemetryRetentionDays    = 30
	defaultTelemetryQueryPreview     = 80
	defaultHookWriteTemplate         = "{{ .safe_text }}"
	defaultCaptureMaxEpisodeAge      = "1m"
	defaultCaptureRetryMin           = "1s"
	defaultCaptureMaxAttempts        = 10
)

type Config struct {
	Version        int                            `json:"version" yaml:"version"`
	Identity       IdentityConfig                 `json:"identity,omitempty" yaml:"identity,omitempty"`
	Providers      map[string]ProviderConfig      `json:"providers" yaml:"providers"`
	RecallProfiles map[string]RecallProfileConfig `json:"recall_profiles,omitempty" yaml:"recall_profiles,omitempty"`
	WriteProfiles  map[string]WriteProfileConfig  `json:"write_profiles,omitempty" yaml:"write_profiles,omitempty"`
	Agents         map[string]AgentConfig         `json:"agents,omitempty" yaml:"agents,omitempty"`
	Telemetry      TelemetryConfig                `json:"telemetry,omitempty" yaml:"telemetry,omitempty"`
	CaptureQueue   CaptureQueueConfig             `json:"capture_queue,omitempty" yaml:"capture_queue,omitempty"`

	Hooks map[string]LegacyHookConfig `json:"hooks,omitempty" yaml:"hooks,omitempty"`
}

type IdentityConfig struct {
	UserID string `json:"user_id,omitempty" yaml:"user_id,omitempty"`
}

type MemoryScopeConfig struct {
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
	ID   string `json:"id,omitempty" yaml:"id,omitempty"`
}

type CaptureQueueConfig struct {
	Path                string         `json:"path,omitempty" yaml:"path,omitempty"`
	MaxEpisodeAge       string         `json:"max_episode_age,omitempty" yaml:"max_episode_age,omitempty"`
	RetryMin            string         `json:"retry_min,omitempty" yaml:"retry_min,omitempty"`
	MaxAttempts         int            `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
	ProviderConcurrency map[string]int `json:"provider_concurrency,omitempty" yaml:"provider_concurrency,omitempty"`
}

type ProviderConfig struct {
	Type               string            `json:"type" yaml:"type"`
	Enabled            bool              `json:"enabled" yaml:"enabled"`
	Path               string            `json:"path,omitempty" yaml:"path,omitempty"`
	APIKey             string            `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	BaseURL            string            `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	Transport          string            `json:"transport,omitempty" yaml:"transport,omitempty"`
	Command            string            `json:"command,omitempty" yaml:"command,omitempty"`
	Args               []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env                map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Timeout            string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	UserID             string            `json:"user_id,omitempty" yaml:"user_id,omitempty"`
	AgentID            string            `json:"agent_id,omitempty" yaml:"agent_id,omitempty"`
	RunID              string            `json:"run_id,omitempty" yaml:"run_id,omitempty"`
	ScoreSemantics     string            `json:"score_semantics,omitempty" yaml:"score_semantics,omitempty"`
	SearchScopePayload string            `json:"search_scope_payload,omitempty" yaml:"search_scope_payload,omitempty"`
	GraphID            string            `json:"graph_id,omitempty" yaml:"graph_id,omitempty"`
	MemCubeID          string            `json:"mem_cube_id,omitempty" yaml:"mem_cube_id,omitempty"`
	SearchMode         string            `json:"search_mode,omitempty" yaml:"search_mode,omitempty"`
	SearchScope        string            `json:"search_scope,omitempty" yaml:"search_scope,omitempty"`
	MaxCharacters      int               `json:"max_characters,omitempty" yaml:"max_characters,omitempty"`
	SourceDescription  string            `json:"source_description,omitempty" yaml:"source_description,omitempty"`
	Infer              *bool             `json:"infer,omitempty" yaml:"infer,omitempty"`

	Read     *bool   `json:"read,omitempty" yaml:"read,omitempty"`
	Write    *bool   `json:"write,omitempty" yaml:"write,omitempty"`
	Required *bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Weight   float64 `json:"weight,omitempty" yaml:"weight,omitempty"`
}

type ProviderRouteConfig struct {
	Name       string                 `json:"name" yaml:"name"`
	Required   bool                   `json:"required" yaml:"required"`
	Weight     float64                `json:"weight,omitempty" yaml:"weight,omitempty"`
	Timeout    string                 `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Thresholds *RecallThresholdConfig `json:"thresholds,omitempty" yaml:"thresholds,omitempty"`
}

type RecallProfileConfig struct {
	Providers  []ProviderRouteConfig `json:"providers,omitempty" yaml:"providers,omitempty"`
	MaxResults int                   `json:"max_results,omitempty" yaml:"max_results,omitempty"`
	Thresholds RecallThresholdConfig `json:"thresholds,omitempty" yaml:"thresholds,omitempty"`
	Ranking    RankingConfig         `json:"ranking,omitempty" yaml:"ranking,omitempty"`
	Tiers      []string              `json:"tiers,omitempty" yaml:"tiers,omitempty"`
}

type RecallThresholdConfig struct {
	MinRelevance float64 `json:"min_relevance,omitempty" yaml:"min_relevance,omitempty"`
	MinScore     float64 `json:"min_score,omitempty" yaml:"min_score,omitempty"`
}

type RankingConfig struct {
	Type         string  `json:"type,omitempty" yaml:"type,omitempty"`
	RecencyBoost float64 `json:"recency_boost,omitempty" yaml:"recency_boost,omitempty"`
}

type WriteProfileConfig struct {
	Providers    []ProviderRouteConfig `json:"providers,omitempty" yaml:"providers,omitempty"`
	Tier         string                `json:"tier,omitempty" yaml:"tier,omitempty"`
	ExpiresAfter string                `json:"expires_after,omitempty" yaml:"expires_after,omitempty"`
	Scope        MemoryScopeConfig     `json:"scope,omitempty" yaml:"scope,omitempty"`
}

type AgentConfig struct {
	Enabled               bool                       `json:"enabled" yaml:"enabled"`
	AgentID               string                     `json:"agent_id,omitempty" yaml:"agent_id,omitempty"`
	PassiveWriteStartedAt string                     `json:"passive_write_started_at,omitempty" yaml:"passive_write_started_at,omitempty"`
	Integration           AgentIntegrationConfig     `json:"integration,omitempty" yaml:"integration,omitempty"`
	ActiveRecall          ActiveRecallConfig         `json:"active_recall,omitempty" yaml:"active_recall,omitempty"`
	Hooks                 map[string]AgentHookConfig `json:"hooks,omitempty" yaml:"hooks,omitempty"`
}

// AgentIntegrationConfig records which installation surface owns the agent's
// lifecycle hooks. An empty owner preserves the original paxm-managed
// behavior. The explicit owner prevents a Codex plugin and paxm setup from
// registering the same hooks twice.
type AgentIntegrationConfig struct {
	Owner string `json:"owner,omitempty" yaml:"owner,omitempty"`
}

const (
	IntegrationOwnerPaxm         = "paxm"
	IntegrationOwnerCodexPlugin  = "codex-plugin"
	IntegrationOwnerClaudePlugin = "claude-plugin"
)

type ActiveRecallConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Profile string `json:"profile,omitempty" yaml:"profile,omitempty"`
	Output  string `json:"output,omitempty" yaml:"output,omitempty"`
}

type AgentHookConfig struct {
	Recall HookRecallConfig `json:"recall,omitempty" yaml:"recall,omitempty"`
	Write  HookWriteConfig  `json:"write,omitempty" yaml:"write,omitempty"`
}

type HookRecallConfig struct {
	Enabled       bool                `json:"enabled" yaml:"enabled"`
	Profile       string              `json:"profile,omitempty" yaml:"profile,omitempty"`
	QueryTemplate string              `json:"query_template,omitempty" yaml:"query_template,omitempty"`
	MaxResults    int                 `json:"max_results,omitempty" yaml:"max_results,omitempty"`
	Timeout       string              `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	TimeoutExtra  string              `json:"timeout_extra,omitempty" yaml:"timeout_extra,omitempty"`
	Output        string              `json:"output,omitempty" yaml:"output,omitempty"`
	Insertion     HookInsertionConfig `json:"insertion,omitempty" yaml:"insertion,omitempty"`
	Initial       *HookInitialRecall  `json:"initial,omitempty" yaml:"initial,omitempty"`
}

type HookInitialRecall struct {
	Enabled       bool                `json:"enabled" yaml:"enabled"`
	Profile       string              `json:"profile,omitempty" yaml:"profile,omitempty"`
	QueryTemplate string              `json:"query_template,omitempty" yaml:"query_template,omitempty"`
	MaxResults    int                 `json:"max_results,omitempty" yaml:"max_results,omitempty"`
	Timeout       string              `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	TimeoutExtra  string              `json:"timeout_extra,omitempty" yaml:"timeout_extra,omitempty"`
	Insertion     HookInsertionConfig `json:"insertion,omitempty" yaml:"insertion,omitempty"`
}

type HookInsertionConfig struct {
	MinScore          float64 `json:"min_score,omitempty" yaml:"min_score,omitempty"`
	MaxItems          int     `json:"max_items,omitempty" yaml:"max_items,omitempty"`
	RequireQueryTerms bool    `json:"require_query_terms,omitempty" yaml:"require_query_terms,omitempty"`
}

type HookWriteConfig struct {
	Enabled  bool             `json:"enabled" yaml:"enabled"`
	Profile  string           `json:"profile,omitempty" yaml:"profile,omitempty"`
	Template string           `json:"template,omitempty" yaml:"template,omitempty"`
	Mode     string           `json:"mode,omitempty" yaml:"mode,omitempty"`
	Buffer   HookBufferConfig `json:"buffer,omitempty" yaml:"buffer,omitempty"`
}

type HookBufferConfig struct {
	Enabled    bool `json:"enabled" yaml:"enabled"`
	Flush      bool `json:"flush,omitempty" yaml:"flush,omitempty"`
	FlushCount int  `json:"flush_count,omitempty" yaml:"flush_count,omitempty"`
}

type LegacyHookConfig struct {
	Enabled bool                             `json:"enabled" yaml:"enabled"`
	Events  map[string]LegacyHookEventConfig `json:"events,omitempty" yaml:"events,omitempty"`
}

type LegacyHookEventConfig struct {
	Recall HookRecallConfig `json:"recall" yaml:"recall"`
}

type TelemetryConfig struct {
	Enabled             *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Dir                 string `json:"dir,omitempty" yaml:"dir,omitempty"`
	EventsFile          string `json:"events_file,omitempty" yaml:"events_file,omitempty"`
	MetricsFile         string `json:"metrics_file,omitempty" yaml:"metrics_file,omitempty"`
	MaxEventFileBytes   int64  `json:"max_event_file_bytes,omitempty" yaml:"max_event_file_bytes,omitempty"`
	MaxEventFiles       int    `json:"max_event_files,omitempty" yaml:"max_event_files,omitempty"`
	RetentionDays       int    `json:"retention_days,omitempty" yaml:"retention_days,omitempty"`
	CaptureQueryPreview *bool  `json:"capture_query_preview,omitempty" yaml:"capture_query_preview,omitempty"`
	QueryPreviewChars   int    `json:"query_preview_chars,omitempty" yaml:"query_preview_chars,omitempty"`
}

func DefaultConfigPath() string {
	if path := os.Getenv("PAXM_CONFIG"); path != "" {
		return ExpandPath(path)
	}
	return filepath.Join(defaultConfigDir(), "config.yaml")
}

func defaultConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(ExpandPath(dir), "paxm")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".paxm"
	}
	return filepath.Join(home, ".config", "paxm")
}

func DefaultDataPath() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(ExpandPath(dir), "paxm", "memory.sqlite")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".paxm", "memory.sqlite")
	}
	return filepath.Join(home, ".local", "share", "paxm", "memory.sqlite")
}

func DefaultStateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(ExpandPath(dir), "paxm")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".paxm", "state")
	}
	return filepath.Join(home, ".local", "state", "paxm")
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	path = ExpandPath(path)
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			legacyPath := legacyJSONPath(path)
			if legacyPath != path {
				if legacyFile, legacyErr := os.Open(legacyPath); legacyErr == nil {
					defer func() { _ = legacyFile.Close() }()
					var cfg Config
					if decodeErr := decodeConfig(legacyFile, legacyPath, &cfg); decodeErr != nil {
						return Config{}, decodeErr
					}
					if validateErr := Validate(cfg); validateErr != nil {
						return Config{}, validateErr
					}
					return Normalize(cfg), nil
				}
			}
			return Config{}, fmt.Errorf("%w: %s", ErrConfigMissing, path)
		}
		return Config{}, err
	}
	defer func() { _ = file.Close() }()

	var cfg Config
	if err := decodeConfig(file, path, &cfg); err != nil {
		return Config{}, err
	}
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return Normalize(cfg), nil
}

func Save(path string, cfg Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	if path == "" {
		path = DefaultConfigPath()
	}
	path = ExpandPath(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return encodeConfig(file, path, Normalize(cfg))
}

func Exists(path string) bool {
	if path == "" {
		path = DefaultConfigPath()
	}
	path = ExpandPath(path)
	if _, err := os.Stat(path); err == nil {
		return true
	}
	legacyPath := legacyJSONPath(path)
	if legacyPath == path {
		return false
	}
	_, err := os.Stat(legacyPath)
	return err == nil
}

func decodeConfig(file *os.File, path string, cfg *Config) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return json.NewDecoder(file).Decode(cfg)
	}
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(false)
	return decoder.Decode(cfg)
}

func encodeConfig(file *os.File, path string, cfg Config) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		return encoder.Encode(cfg)
	}
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(cfg)
}

func ProviderRouteRequired(routes []ProviderRouteConfig, provider string) (bool, bool) {
	for _, route := range routes {
		if route.Name == provider {
			return route.Required, true
		}
	}
	return false, false
}

func UpsertProviderRoute(routes []ProviderRouteConfig, provider string, required bool) []ProviderRouteConfig {
	for i, route := range routes {
		if route.Name == provider {
			route.Required = required
			if route.Weight == 0 {
				route.Weight = defaultProviderRouteWeight
			}
			routes[i] = route
			return routes
		}
	}
	return append(routes, ProviderRouteConfig{Name: provider, Required: required, Weight: defaultProviderRouteWeight})
}

func RemoveProviderRoute(routes []ProviderRouteConfig, provider string) []ProviderRouteConfig {
	filtered := routes[:0]
	for _, route := range routes {
		if route.Name != provider {
			filtered = append(filtered, route)
		}
	}
	return filtered
}

func ExpandPath(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if len(path) > 1 && os.IsPathSeparator(path[1]) {
		return filepath.Join(home, path[2:])
	}
	return path
}
