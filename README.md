# gooo-test-frontier

`gooo-test-frontier` maps a semantic change graph to the smallest affected
test frontier and accounts for every test with exactly one of `EXECUTED`,
`REUSED`, `SKIPPED`, or `NOT_OBSERVED`.

The evaluator consumes exact source, toolchain, policy, and test-inventory
digests; semantic change edges; immutable prior test receipts; and an optional
exact before/after performance pair. A cache hit is ignored unless all receipt
identities and the terminal `PASS` result match. Reused tests carry no current
execution duration, so reuse cannot be disguised as a zero-millisecond run.

The implementation is added through the feature pull request after this
The fixed denominator is twelve 1:1 meta activities. It has four activities in
each proof choice (`FOUNDATION`, `COHERENCE`, `REGRESSION`) and four in each
indicator class (`DRIVER`, `OUTCOME`, `GUARDRAIL`). Decisions use
`REFUTED > UNKNOWN > CLOSED`; every UNKNOWN preserves stage, step, reason,
unknown class, next operation, and its minimal blocked-by frontier.

This repository is fixture simulation only. It never changes a target
repository and never executes target tests. GitHub Actions records exact status
counts, invalidated edges, evidence lookup/verification time, build/test/
conformance wall time, peak RSS, physical inventory, output artifacts, and
zero product-authority counters.

Run with an absolute caller-owned output directory:

```text
gooo-test-frontier conformance --output-dir /tmp/gooo-test-frontier-out
```

Time and memory improvements are reported only for the same exact before/after
pair key. Otherwise economy is `UNKNOWN`. Any false-negative counterexample is
`REFUTED`, even when other evidence is missing.
