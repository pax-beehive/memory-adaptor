# paxm Architecture

`paxm` exposes one CLI surface for agent memory while keeping provider setup,
hook installation, and recall policy in user-owned configuration.

## Layers

```text
cmd/paxm
  internal/cli          command parsing and interactive setup
  internal/facade       active recall, hook recall, and writes
  internal/memory       provider interface, routing, ranking, thresholds
  internal/adapters     provider registry
  internal/config       YAML config model and compatibility loading
```

The CLI never talks to concrete providers directly. It loads config, builds the
provider registry/router, and calls the facade.

## Provider Boundary

A memory provider is responsible for:

- connecting to one backing store or service;
- storing memory items;
- searching memory items;
- returning provider-local results with normalized relevance.

Provider relevance should be normalized to `[0, 1]` by the adapter. The router
can then compare hits from different providers without knowing provider-specific
score systems such as keyword ratios, vector distance, cosine similarity, or
vendor-specific ranks.

Provider configuration describes availability and connection details. It should
not decide whether a specific hook or active recall path reads from the provider.

Current provider adapters:

- `local`: local JSONL storage with keyword relevance.
- `zep`: Zep Graph storage via `github.com/getzep/zep-go/v3`; writes text
  episodes and maps graph search results into memory hits.

## Recall Profiles

A recall profile is the policy boundary for reads. It chooses:

- which enabled providers participate;
- whether each provider is required or best effort for that route;
- each provider route weight;
- max result count;
- relevance and final score thresholds;
- ranking behavior.

`min_relevance` filters provider-normalized hits before cross-provider ranking.
`min_score` filters the final merged score after route weight and ranking boosts.

## Write Profiles

A write profile is the policy boundary for writes. It chooses:

- which enabled providers receive writes;
- whether each provider is required or best effort for that write route.

Enabled providers can be used by multiple read and write profiles.

## Agent Entries

An agent entry describes how an agent uses memory. It does not duplicate provider
configuration.

- `active_recall` is used by explicit `paxm recall --query ...` calls.
- `hooks.*.recall` is passive recall triggered by agent hooks.
- `hooks.*.write` is reserved for passive writes from future hook events.

Both active recall and hook recall point at recall profiles.

## Hook Behavior

V1 hook behavior is passive recall only. The Codex user prompt hook calls:

```text
paxm --config PATH recall --hook-event --json
```

The hook event is converted into a recall query using the hook recall template.
The hook does not write memory in V1 unless a future write hook is explicitly
enabled and implemented.
