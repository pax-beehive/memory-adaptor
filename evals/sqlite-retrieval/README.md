# SQLite retrieval challenge

These deterministic suites define the quality frontier for paxm's default local
provider. They deliberately target lightweight, explicit, rg-like retrieval
rather than embeddings or hidden model-based query expansion.

`suite.json` covers conservative English morphology, a separately identified
bounded product-alias vocabulary, CJK substring retrieval, identifiers in both
split directions (including paths, versions, and error codes), strict all-term
suppression, relaxed fallback when no all-term result exists, and suppression of
long transcript-like distractors. Every substring and identifier case includes a
near distractor so broad matching alone cannot pass.

`workspace-suite.json` is separate because workspace isolation is a correctness
property, not an aggregate relevance tradeoff. Its target budget requires every
case to pass with zero false positives.

The main challenge remains a non-gating report while explicit query planning is
implemented. Workspace isolation has graduated to a hard CI gate alongside the
existing 100-case baseline.

On 2026-07-12, the lightweight analyzer raised the main challenge from 8 to 24
of 32 passing cases, with Recall@K 1.000, Precision@K 0.859, MRR 0.922, and
false-positive rate 0.256. Identifier splitting, bounded aliases, conservative
morphology, CJK substrings, relaxed fallback, and workspace isolation all pass.
The remaining failures cover strict partial suppression and long-noise ranking.
The workspace suite passes 5 of 5 with zero false positives, and the existing
baseline passes 100 of 100.

Run the challenges:

```sh
paxm eval run --suite evals/sqlite-retrieval/suite.json --gate none
paxm eval run --suite evals/sqlite-retrieval/workspace-suite.json --gate quality \
  --budget evals/sqlite-retrieval/workspace-target-budget.json
```

Regenerate both suites only after intentionally changing the source matrix:

```sh
go run ./evals/sqlite-retrieval/generate
```

Full graduation requires all three gates. CI already enforces the baseline and
workspace commands; the aggregate challenge remains the final target:

```sh
paxm eval run --suite evals/baseline --gate quality \
  --budget evals/baseline/budget.json
paxm eval run --suite evals/sqlite-retrieval/suite.json --gate quality \
  --budget evals/sqlite-retrieval/target-budget.json
paxm eval run --suite evals/sqlite-retrieval/workspace-suite.json --gate quality \
  --budget evals/sqlite-retrieval/workspace-target-budget.json
```
