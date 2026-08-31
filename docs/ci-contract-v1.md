# CI and release contract v1

GitHub Actions runs Go 1.27 formatting, build, test, vet, compile, conformance,
deterministic replay, and artifact audits. Local Go validation is intentionally
outside this repository process.

The conformance job runs nine canonical fixture cases: three `CLOSED`, three
`UNKNOWN`, and three `REFUTED`. Every case emits `plan.json`, `receipt.json`,
and `human-report.md` under the runner's temporary output root. Failed attempts
remain in an append-only NDJSON log and are uploaded with the other evidence;
failure is a counterexample, never a successful observation.

The runtime receipt reports exact total, executed, reused, skipped, and
not-observed counts and verifies their sum. It also reports invalidated edges,
evidence lookup/verification milliseconds, build/test/conformance milliseconds,
peak RSS, Go and Gooo physical lines, files and directories excluding the root
README, output artifact count, and `repository_writes=0`,
`local_test_executions=0`, `cross_project_required_gates=0`.

Main starts with exactly `.gitignore`, `LICENSE`, and `README.md`. Feature work
is delivered through one pull request. Any follow-up changes are CI-only,
then main is validated again before the annotated immutable `v0.1.0` release.
