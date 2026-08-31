# RFC: test frontier and test economy evidence

The existing verification-reuse protocol proves whether whole-run evidence has
matching identity. The existing improvement-frontier protocol schedules causal
operations. Neither protocol owns the test-granular question: which tests are
invalidated by a semantic change, how many tests were actually executed or
reused, and whether build/test cost changed under a same-key before/after pair.

This repository owns only that boundary. It consumes immutable evidence and
produces read-only receipts. It does not suppress CI, mutate a subject, or rank
tests. Missing observations remain distinct from policy skips, and false
negatives are preserved as refutations.
