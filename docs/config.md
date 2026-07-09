# paxm YAML Config

Default config path for new installs:

```text
~/.config/paxm/config.yaml
```

`paxm` can still load legacy JSON configs for compatibility, but setup writes
YAML unless an explicit `.json` path is provided.

## Shape

```yaml
version: 1

providers:
  local:
    type: local
    enabled: true
    path: ~/.local/share/paxm/memory.jsonl

  mem0:
    type: mem0
    enabled: false
    api_key: "plain-text-api-key"

recall_profiles:
  default:
    providers:
      - name: local
        required: false
        weight: 1.0
    max_results: 8
    thresholds:
      min_relevance: 0.25
      min_score: 0.25
    ranking:
      type: weighted_relevance
      recency_boost: 0

write_profiles:
  default:
    providers:
      - name: local
        required: false

agents:
  codex:
    enabled: true
    active_recall:
      enabled: true
      profile: default
      output: markdown
    hooks:
      user_prompt:
        recall:
          enabled: true
          profile: default
          query_template: "{{ .prompt }}"
          output: markdown
        write:
          enabled: false
          profile: default
          template: "{{ .prompt }}"
          mode: prompt
```

## Providers

`providers` declares provider instances and connection details.

Fields:

- `type`: adapter type, such as `local`.
- `enabled`: whether this provider can be used by profiles.
- `path`: local provider JSONL path.
- `api_key`: optional plain-text API key for remote providers.

V1 ships with the `local` provider. Additional provider config fields are
accepted so setup and docs can model future remote providers before adapters
exist.

## Recall Profiles

`recall_profiles` defines read strategy.

Provider route fields:

- `name`: provider instance name from `providers`.
- `required`: when true, a provider error fails the recall command.
- `weight`: multiplier applied after provider relevance normalization.

Threshold fields:

- `min_relevance`: provider-normalized relevance threshold before merge.
- `min_score`: final score threshold after weight and ranking boosts.

Ranking fields:

- `type`: currently `weighted_relevance`.
- `recency_boost`: optional boost added from item age when `created_at` exists.

## Write Profiles

`write_profiles` defines write strategy. `paxm remember` uses the `default`
write profile unless another profile is selected.

## Agents

`agents.codex.active_recall` controls explicit recall calls.

`agents.codex.hooks.user_prompt.recall` controls passive recall from the Codex
user prompt hook.

`agents.codex.hooks.user_prompt.write` is present in the config model for future
passive write behavior, but V1 does not install or run write hooks.
