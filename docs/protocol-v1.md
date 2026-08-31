# Semantic-change test frontier protocol v1

This protocol is deliberately narrower than whole-run evidence reuse and
general operation scheduling. It answers which individual tests are reached by
changed semantic units, whether an immutable prior receipt is safe to reuse,
and whether test-economy measurements are comparable.

The fixture binds source, toolchain, policy, test-inventory, and semantic graph
digests. Directed semantic change edges are traversed from `changed_units`; the
evaluator records one shortest deterministic edge path per reached test. The
union of those paths is the minimal invalidation frontier and its exact edge
count.

Each inventory item receives exactly one status: `EXECUTED` when the reached
test has an observed wall time and peak RSS; `REUSED` when the target is not
reached and the prior receipt exactly matches all identities, is immutable, and
has terminal `PASS`; `SKIPPED` when policy excludes it; and `NOT_OBSERVED` when
neither safe reuse evidence nor an execution observation is available.

The four counts must sum exactly to total tests. Non-executed statuses never
carry current execution metrics. A cache hit is not an observation and never
creates `REUSED` by itself.

Before and after snapshots must have identical scenario, source, toolchain,
policy, and test-inventory pair keys, and those keys must match current input.
The exact comparisons are build wall time, test wall time, conformance wall
time, and peak RSS. A missing or mismatched pair is `UNKNOWN`, never an
improvement claim. Any counterexample that expects invalidation but observes no
invalidation is `REFUTED` regardless of cache state or missing measurements.

Release immutability has an external-authority boundary. The GitHub Releases
API `immutable` field and the repository immutable-releases setting are
authoritative over a manifest's self-asserted `immutable` value. A manifest
that says `immutable=true` while the platform returns `immutable=false` is the
canonical `SELF_ASSERTED_IMMUTABILITY_CONTRADICTED_BY_PLATFORM` `REFUTED` case;
it cannot close through the manifest alone.
