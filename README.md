# paxm

`paxm` is a Go CLI for giving agents a stable memory recall surface while leaving setup, API keys, hooks, and provider policy under user control.

## V1 Shape

```text
Human setup:
  paxm setup  # choose providers and agent hooks interactively

Agent active recall:
  paxm recall --query "what did we decide?" --json

Hook passive recall:
  installed hook shim -> paxm recall --hook-event --json
```

The CLI command layer does not talk to concrete memory providers directly. Commands call the facade, the facade calls the memory router, and the router fans out to enabled providers.

```text
cmd/paxm
  internal/cli
  internal/facade
  internal/memory        provider interface and multi-provider router
  internal/adapters      provider registry
  internal/adapters/local
  internal/config
```

## Quick Start

```bash
go build -o /tmp/paxm ./cmd/paxm
/tmp/paxm setup
/tmp/paxm remember --text "paxm supports hook passive recall"
/tmp/paxm recall --query "passive recall"
```

For a project-local config during development:

```bash
/tmp/paxm --config /tmp/paxm-dev/config.yaml setup --force
/tmp/paxm --config /tmp/paxm-dev/config.yaml remember --text "enabled providers can read and write"
printf '{"prompt":"enabled providers"}' | /tmp/paxm --config /tmp/paxm-dev/config.yaml recall --hook-event --json
```

## Config

Default config path:

```text
~/.config/paxm/config.yaml
```

V1 ships with a local JSONL provider so the full flow works without external API keys.
The CLI can load legacy JSON configs, but new setup writes YAML by default.

```yaml
version: 1

providers:
  local:
    type: local
    enabled: true
    path: ~/.local/share/paxm/memory.jsonl

  zep:
    type: zep
    enabled: false
    api_key: "plain-text-zep-api-key"
    user_id: todd
    search_scope: episodes

recall_profiles:
  default:
    providers:
      - name: local
        required: true
        weight: 1
    max_results: 8
    thresholds:
      min_relevance: 0.25
      min_score: 0.25
    ranking:
      type: weighted_relevance

write_profiles:
  default:
    providers:
      - name: local
        required: true

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
          max_results: 8
          output: markdown
        write:
          enabled: false
          profile: default
          template: "{{ .prompt }}"
          mode: prompt
```

Multiple enabled providers are supported by configuration. Recall profiles decide
which providers are read, how provider relevance is weighted, and what
thresholds are applied. Write profiles decide which providers are written.
Optional provider failures are returned as provider errors; required provider
failures fail the command.

Remote provider configs may include a plain-text `api_key` field. Zep is
supported with `type: zep` using `github.com/getzep/zep-go/v3`; configure
exactly one of `user_id` or `graph_id`.

`paxm setup` is the interactive entry point for changing provider and hook choices. It uses numbered selectors for memory providers and agent hooks, then writes the paxm config, installs selected hook shims, and registers Codex hooks in the user-level Codex config.

For Codex, setup writes a shim under the paxm config directory:

```text
~/.config/paxm/hooks/codex-user_prompt
```

It also updates:

```text
~/.codex/config.toml
```

The shim expects a hook event JSON object on stdin and calls `paxm recall --hook-event --json`. Codex may still require you to review and trust the new non-managed hook with `/hooks` before it runs.
