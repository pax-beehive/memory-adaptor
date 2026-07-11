# Conversation-to-write baseline

This version 1 suite contains 40 deterministic SQLite cases: five sanitized
topic variants across eight scenario families. It exercises Codex, Claude Code,
and Pi writes for user input, turn completion, successful and failed tool use,
mixed visible messages, reasoning suppression, and metadata preservation.
Every scenario has normalized user and assistant history, a separate normalized
hook-event message payload, and a seeded harmful recall distractor. Scenarios
whose real hook supplies complete turn history, such as Pi `turn_end`, set
`include_history` and make successful capture depend on those historical turns.

Each case sends normalized turns through the production `HookWriteItem` and
`IngestBatch` path, then performs a later active recall through the production
facade. A case passes only when:

- every expected visible fragment is present in the rendered write;
- no forbidden reasoning or analysis fragment is present;
- hook, workspace, session, and source metadata survive both write and recall;
- a later query returns a hit containing all expected fragments;
- the explicitly forbidden distractor is not returned.

The report includes write recall, write precision, forbidden-fragment insertion
rate, recall result count, write and recall latency totals, and returned
recall-content size. Write recall is the share of expected fragments retained.
Write precision divides retained expected fragments by retained expected plus
forbidden fragments. The write false-positive rate is the share of forbidden
candidates that appeared. These are deterministic capture-quality signals, not
a semantic memory extraction benchmark.

Run the suite with:

```sh
paxm eval run --suite evals/conversation-write
paxm eval run --suite evals/conversation-write --json
```

Regenerate `suite.json` after intentionally changing the source matrix:

```sh
go run ./evals/conversation-write/generate
```
