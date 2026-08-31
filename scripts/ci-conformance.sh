#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(pwd)"
run_id="${GITHUB_RUN_ID:-local-$(date -u +%Y%m%dT%H%M%SZ)}"
output_root="${CI_OUTPUT_ROOT:-${RUNNER_TEMP:-/tmp}/gooo-test-frontier-${run_id}}"
mkdir -p "$output_root/logs" "$output_root/conformance-a" "$output_root/conformance-b"
attempts="$output_root/failed-attempts.ndjson"
: > "$attempts"
failed=0

phase() {
	local name="$1"
	shift
	local log="$output_root/logs/${name}.log"
	local started ended phase_exit
	started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	set +e
	"$@" >"$log" 2>&1
	phase_exit=$?
	set -e
	ended="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	jq -cn --arg phase "$name" --arg status "$([ "$phase_exit" -eq 0 ] && echo SUCCESS || echo FAILED)" --arg started "$started" --arg ended "$ended" --arg log "$log" --argjson exit_code "$phase_exit" '{phase:$phase,status:$status,started_at:$started,ended_at:$ended,exit_code:$exit_code,log:$log}' >> "$attempts"
	cat "$log"
	if [ "$phase_exit" -ne 0 ]; then
		failed=1
	fi
}

build_time="$output_root/build.time"
test_time="$output_root/test.time"
conformance_time="$output_root/conformance.time"
test_json="$output_root/go-test.json"
binary="$output_root/gooo-test-frontier"
ir_a="$output_root/semantic-ir.json"
generated_go_a="$output_root/semantic.gooo.go"
ir_b="$output_root/semantic-ir-b.json"
generated_go_b="$output_root/semantic.gooo-b.go"

phase toolchain bash -c 'test "$(go env GOVERSION)" = "go1.27.0" && test "$(go env GOOS)" = "linux" && test "$(go env GOARCH)" = "amd64"'
phase format bash -c '
	set -Eeuo pipefail
	files="$(git ls-files "*.go")"
	if [ -n "$files" ]; then
		bad="$(gofmt -l $files)"
	else
		bad=""
	fi
	if [ -n "$bad" ]; then
		echo "$bad"
		exit 1
	fi
'
phase build /usr/bin/time -f '%e %M' -o "$build_time" go build -trimpath -o "$binary" ./cmd/gooo-test-frontier
phase test /usr/bin/time -f '%e %M' -o "$test_time" go test -json -count=1 ./... > "$test_json"
phase vet go vet ./...
phase compile-a "$binary" compile --source examples/test-frontier.gooo --contract contracts/test-frontier-denominator-v1.json --output-ir "$ir_a" --output-go "$generated_go_a"
phase generated-ir cmp "$ir_a" internal/generated/semantic-ir.json
phase generated-go cmp "$generated_go_a" internal/generated/semantic.gooo.go
phase conformance-a /usr/bin/time -f '%e %M' -o "$conformance_time" "$binary" conformance --root "$repo_root" --source examples/test-frontier.gooo --contract contracts/test-frontier-denominator-v1.json --corpus examples/canonical-corpus.json --output-dir "$output_root/conformance-a"
phase compile-b "$binary" compile --source examples/test-frontier.gooo --contract contracts/test-frontier-denominator-v1.json --output-ir "$ir_b" --output-go "$generated_go_b"
phase deterministic-ir cmp "$ir_a" "$ir_b"
phase deterministic-go cmp "$generated_go_a" "$generated_go_b"
phase conformance-b /usr/bin/time -f '%e %M' -o "$conformance_time-b" "$binary" conformance --root "$repo_root" --source examples/test-frontier.gooo --contract contracts/test-frontier-denominator-v1.json --corpus examples/canonical-corpus.json --output-dir "$output_root/conformance-b"
phase deterministic-plan diff -ru "$output_root/conformance-a" "$output_root/conformance-b"
phase artifact-audit bash -c '
	set -Eeuo pipefail
	test "$(find "$1" -type f -name receipt.json | wc -l | tr -d " ")" -eq 9
	test -f "$1/conformance-index.json"
	jq -e ".states.CLOSED >= 3 and .states.UNKNOWN >= 3 and .states.REFUTED >= 3" "$1/conformance-index.json" >/dev/null
	for case_dir in "$1"/*; do
		test -d "$case_dir" || continue
		test -f "$case_dir/plan.json"
		test -f "$case_dir/receipt.json"
		test -f "$case_dir/human-report.md"
		jq -e ".execution_counts.total == (.execution_counts.executed + .execution_counts.reused + .execution_counts.skipped + .execution_counts.not_observed) and .product_authority == {repository_writes:0,local_test_executions:0,cross_project_required_gates:0}" "$case_dir/receipt.json" >/dev/null
		jq -e "all(.activities[]; .state != \"UNKNOWN\" or (.unknown.stage != \"\" and .unknown.step != \"\" and .unknown.reason != \"\" and .unknown.unknown_class != \"\" and .unknown.next_operation != \"\" and (.unknown.blocked_by|length) > 0))" "$case_dir/plan.json" >/dev/null
	done
' _ "$output_root/conformance-a"
phase repository-audit bash -c 'test -z "$(git status --porcelain --untracked-files=all)"'

read_metric() {
	local file="$1"
	if [ -s "$file" ]; then
		awk '{print $1 " " $2}' "$file"
	else
		echo "0 0"
	fi
}

read -r build_seconds build_rss <<< "$(read_metric "$build_time")"
read -r test_seconds test_rss <<< "$(read_metric "$test_time")"
read -r conformance_seconds conformance_rss <<< "$(read_metric "$conformance_time")"
build_wall_ms="$(awk -v value="$build_seconds" 'BEGIN { printf "%d", (value * 1000) + 0.5 }')"
test_wall_ms="$(awk -v value="$test_seconds" 'BEGIN { printf "%d", (value * 1000) + 0.5 }')"
conformance_wall_ms="$(awk -v value="$conformance_seconds" 'BEGIN { printf "%d", (value * 1000) + 0.5 }')"
peak_rss_kib="$(awk -v a="$build_rss" -v b="$test_rss" -v c="$conformance_rss" 'BEGIN { value=a; if (b>value) value=b; if (c>value) value=c; print value+0 }')"

receipt_paths="$(find "$output_root/conformance-a" -type f -name receipt.json -print)"
if [ -n "$receipt_paths" ]; then
	read -r total_tests tests_executed tests_reused tests_skipped tests_not_observed invalidated_edges lookup_ms verification_ms <<EOF
$(printf '%s\n' "$receipt_paths" | xargs jq -s 'reduce .[] as $r ({total:0,executed:0,reused:0,skipped:0,not_observed:0,edges:0,lookup:0,verification:0}; .total += $r.execution_counts.total | .executed += $r.execution_counts.executed | .reused += $r.execution_counts.reused | .skipped += $r.execution_counts.skipped | .not_observed += $r.execution_counts.not_observed | .edges += $r.invalidated_edge_count | .lookup += $r.evidence_timing.lookup_ms | .verification += $r.evidence_timing.verification_ms) | [.total,.executed,.reused,.skipped,.not_observed,.edges,.lookup,.verification] | @tsv' | tr '\t' ' ')
EOF
else
	total_tests=0; tests_executed=0; tests_reused=0; tests_skipped=0; tests_not_observed=0; invalidated_edges=0; lookup_ms=0; verification_ms=0
fi

if [ -s "$test_json" ]; then
	go_tests_discovered="$(jq -s '[.[] | select(.Action == "run" and .Test != null)] | length' "$test_json")"
	go_tests_executed="$(jq -s '[.[] | select((.Action == "pass" or .Action == "fail") and .Test != null)] | length' "$test_json")"
	go_tests_skipped="$(jq -s '[.[] | select(.Action == "skip" and .Test != null)] | length' "$test_json")"
else
	go_tests_discovered=0; go_tests_executed=0; go_tests_skipped=0
fi
output_artifacts="$(find "$output_root/conformance-a" -type f | wc -l | tr -d ' ')"
file_count="$(find . -type f -not -path './.git/*' -not -path './README.md' | wc -l | tr -d ' ')"
directory_count="$(find . -type d -not -path './.git' -not -path './.git/*' | wc -l | tr -d ' ')"
physical_lines="$(find . -type f -not -path './.git/*' -not -path './README.md' -print0 | xargs -0 -r awk '{ total++ } END { print total + 0 }')"
go_files="$(find . -type f -name '*.go' -not -path './.git/*' | wc -l | tr -d ' ')"
go_physical_lines="$(find . -type f -name '*.go' -not -path './.git/*' -print0 | xargs -0 -r awk '{ total++ } END { print total + 0 }')"
gooo_files="$(find . -type f -name '*.gooo' -not -path './.git/*' | wc -l | tr -d ' ')"
gooo_physical_lines="$(find . -type f -name '*.gooo' -not -path './.git/*' -print0 | xargs -0 -r awk '{ total++ } END { print total + 0 }')"
ci_job_id="${GITHUB_JOB:-conformance}"

jq -S -n \
	--arg schema 'gooo/test-frontier/ci-runtime/v1' \
	--arg run_id "$run_id" \
	--arg job_id "$ci_job_id" \
	--argjson total_tests "$total_tests" \
	--argjson tests_executed "$tests_executed" \
	--argjson tests_reused "$tests_reused" \
	--argjson tests_skipped "$tests_skipped" \
	--argjson tests_not_observed "$tests_not_observed" \
	--argjson invalidated_edges "$invalidated_edges" \
	--argjson lookup_ms "$lookup_ms" \
	--argjson verification_ms "$verification_ms" \
	--argjson build_wall_ms "$build_wall_ms" \
	--argjson test_wall_ms "$test_wall_ms" \
	--argjson conformance_wall_ms "$conformance_wall_ms" \
	--argjson peak_rss_kib "$peak_rss_kib" \
	--argjson output_artifacts "$output_artifacts" \
	--argjson go_tests_discovered "$go_tests_discovered" \
	--argjson go_tests_executed "$go_tests_executed" \
	--argjson go_tests_skipped "$go_tests_skipped" \
	--argjson directories "$directory_count" \
	--argjson files "$file_count" \
	--argjson physical_lines "$physical_lines" \
	--argjson go_files "$go_files" \
	--argjson go_physical_lines "$go_physical_lines" \
	--argjson gooo_files "$gooo_files" \
	--argjson gooo_physical_lines "$gooo_physical_lines" \
	--argjson failed "$failed" \
	'{schema:$schema,ci_run_id:$run_id,ci_job_id:$job_id,tests:{total:$total_tests,executed:$tests_executed,reused:$tests_reused,skipped:$tests_skipped,not_observed:$tests_not_observed,sum_is_exactly_total:($total_tests == ($tests_executed+$tests_reused+$tests_skipped+$tests_not_observed))},invalidated_edge_count:$invalidated_edges,evidence:{lookup_ms:$lookup_ms,verification_ms:$verification_ms},performance:{build_wall_ms:$build_wall_ms,test_wall_ms:$test_wall_ms,conformance_wall_ms:$conformance_wall_ms,peak_rss_kib:$peak_rss_kib},go_test_observation:{discovered:$go_tests_discovered,executed:$go_tests_executed,skipped:$go_tests_skipped},inventory:{directories:$directories,files:$files,physical_lines:$physical_lines,go_files:$go_files,go_physical_lines:$go_physical_lines,gooo_files:$gooo_files,gooo_physical_lines:$gooo_physical_lines,root_readme_excluded:true},output_artifacts:$output_artifacts,product_authority:{repository_writes:0,local_test_executions:0,cross_project_required_gates:0},failed_run_is_counterexample:($failed != 0)}' \
	> "$output_root/runtime-receipt.json"

printf 'CI_OUTPUT_ROOT=%s\n' "$output_root"
if [ "$failed" -ne 0 ]; then exit 1; fi
