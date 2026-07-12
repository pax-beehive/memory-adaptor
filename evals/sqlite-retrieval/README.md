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

The current SQLite implementation is expected to fail these suites. CI runs both
with `--gate none`, so execution errors fail the job while retrieval-quality
failures remain visible reports. `target-budget.json` and
`workspace-target-budget.json` record the intended graduation criteria; neither
is enforced until the implementation reaches them without regressing the
existing 100-case baseline.

On 2026-07-12, the current implementation passed 8 of 32 main-challenge cases
with Recall@K 0.500, Precision@K 0.359, MRR 0.422, and false-positive rate
0.515. It passed 0 of 5 workspace cases, with false-positive rate 0.500. The
existing baseline still passed 100 of 100 cases.

Run the challenges:

```sh
paxm eval run --suite evals/sqlite-retrieval/suite.json --gate none
paxm eval run --suite evals/sqlite-retrieval/workspace-suite.json --gate none
```

Regenerate both suites only after intentionally changing the source matrix:

```sh
go run ./evals/sqlite-retrieval/generate
```

Graduation requires all three gates. The existing baseline remains a mandatory
regression gate when the challenge moves from reporting to enforcement in CI:

```sh
paxm eval run --suite evals/baseline --gate quality \
  --budget evals/baseline/budget.json
paxm eval run --suite evals/sqlite-retrieval/suite.json --gate quality \
  --budget evals/sqlite-retrieval/target-budget.json
paxm eval run --suite evals/sqlite-retrieval/workspace-suite.json --gate quality \
  --budget evals/sqlite-retrieval/workspace-target-budget.json
```
