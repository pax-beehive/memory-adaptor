# Memory lifecycle evaluation

This deterministic 40-case SQLite suite exercises production hook writes and
later active/passive recall across a fresh runtime load. It covers passive and
active recall after writes, duplicate-write consolidation, and recall-echo
suppression after restart.

```sh
go run ./evals/lifecycle/generate
go run ./cmd/paxm eval run --suite evals/lifecycle --budget evals/lifecycle/budget.json
```
