package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrConfigMissing = errors.New("paxm config is missing")

type Config struct {
	Version   int                       `json:"version"`
	Providers map[string]ProviderConfig `json:"providers"`
	Hooks     map[string]HookConfig     `json:"hooks,omitempty"`
}

type ProviderConfig struct {
	Type      string  `json:"type"`
	Enabled   bool    `json:"enabled"`
	Read      bool    `json:"read"`
	Write     bool    `json:"write"`
	Required  bool    `json:"required"`
	Path      string  `json:"path,omitempty"`
	APIKeyEnv string  `json:"api_key_env,omitempty"`
	Weight    float64 `json:"weight,omitempty"`
}

type HookConfig struct {
	Enabled bool                       `json:"enabled"`
	Events  map[string]HookEventConfig `json:"events,omitempty"`
}

type HookEventConfig struct {
	Recall HookRecallConfig `json:"recall"`
}

type HookRecallConfig struct {
	Enabled       bool   `json:"enabled"`
	QueryTemplate string `json:"query_template,omitempty"`
	MaxResults    int    `json:"max_results,omitempty"`
	Output        string `json:"output,omitempty"`
}

func DefaultConfigPath() string {
	if path := os.Getenv("PAXM_CONFIG"); path != "" {
		return ExpandPath(path)
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(ExpandPath(dir), "paxm", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".paxm", "config.json")
	}
	return filepath.Join(home, ".config", "paxm", "config.json")
}

func DefaultDataPath() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(ExpandPath(dir), "paxm", "memory.jsonl")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".paxm", "memory.jsonl")
	}
	return filepath.Join(home, ".local", "share", "paxm", "memory.jsonl")
}

func DefaultConfig(configPath string) Config {
	configPath = ExpandPath(configPath)
	dataPath := DefaultDataPath()
	if configPath != "" && configPath != DefaultConfigPath() {
		dataPath = filepath.Join(filepath.Dir(configPath), "memory.jsonl")
	}
	return Config{
		Version: 1,
		Providers: map[string]ProviderConfig{
			"local": {
				Type:     "local",
				Enabled:  true,
				Read:     true,
				Write:    true,
				Required: true,
				Path:     dataPath,
				Weight:   1,
			},
		},
		Hooks: map[string]HookConfig{
			"codex": {
				Enabled: true,
				Events: map[string]HookEventConfig{
					"user_prompt": {
						Recall: HookRecallConfig{
							Enabled:       true,
							QueryTemplate: "{{ .prompt }}",
							MaxResults:    8,
							Output:        "markdown",
						},
					},
				},
			},
		},
	}
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	path = ExpandPath(path)
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("%w: %s", ErrConfigMissing, path)
		}
		return Config{}, err
	}
	defer file.Close()

	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return Config{}, err
	}
	return Normalize(cfg), nil
}

func Save(path string, cfg Config) error {
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
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(Normalize(cfg))
}

func Exists(path string) bool {
	if path == "" {
		path = DefaultConfigPath()
	}
	_, err := os.Stat(ExpandPath(path))
	return err == nil
}

func Normalize(cfg Config) Config {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}
	for name, provider := range cfg.Providers {
		if provider.Weight == 0 {
			provider.Weight = 1
		}
		if provider.Path != "" {
			provider.Path = ExpandPath(provider.Path)
		}
		cfg.Providers[name] = provider
	}
	if cfg.Hooks == nil {
		cfg.Hooks = make(map[string]HookConfig)
	}
	return cfg
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
