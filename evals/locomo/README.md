# LoCoMo text-QA retrieval benchmark

This benchmark imports the official LoCoMo `locomo10.json` dataset and measures
whether a configured paxm provider retrieves the annotated dialogue evidence
for category 1-4 text questions. Category 5 adversarial questions and the
multimodal generation and summarization tasks are intentionally excluded from
this retrieval benchmark.

Download `locomo10.json` from the
[official SNAP Research repository](https://github.com/snap-research/locomo/tree/main/data),
then run:

```bash
paxm --config ~/.config/paxm/config.yaml eval run locomo \
  --dataset /path/to/locomo10.json \
  --provider sqlite \
  --output locomo-sqlite.json
```

Remote providers are fail-closed. Each conversation receives a unique eval
scope, every returned memory ref is recorded in an atomic local manifest, and
cleanup runs after success or failure. Mem0 uses an isolated `run_id` with
inference disabled so evidence IDs survive. Zep uses an isolated graph and
episode search. SQLite uses a disposable database. JSON-RPC providers must be
run with `--keep-memory` until their protocol exposes a reliable cleanup
capability.

Use a settle duration for asynchronously indexed providers:

```bash
paxm eval run locomo --dataset locomo10.json --provider zep --settle 10s
```

Interrupted runs can be recovered from their manifests:

```bash
paxm eval cleanup --run RUN_ID
paxm eval cleanup --stale
```

`--keep-memory` is an explicit debugging escape hatch. An explicit
`eval cleanup --run RUN_ID` later overrides it.

The report includes overall and per-category Recall@K, Precision@K, MRR,
pass/fail counts, per-question hit IDs, and execution failures. It evaluates
the memory retrieval layer only; answer-model and LLM-judge quality are not
mixed into these scores.
