package frontier

import "sort"

var activityDefinitions = []ContractCell{
	{Ordinal: 1, ID: "BIND_SOURCE_DIGEST", Activity: "BindSourceDigest", Stage: "BINDING", Step: "VERIFY_SOURCE_DIGEST", ProofChoice: "FOUNDATION", IndicatorClass: "DRIVER"},
	{Ordinal: 2, ID: "BIND_TOOLCHAIN_DIGEST", Activity: "BindToolchainDigest", Stage: "BINDING", Step: "VERIFY_TOOLCHAIN_DIGEST", ProofChoice: "FOUNDATION", IndicatorClass: "DRIVER"},
	{Ordinal: 3, ID: "BIND_POLICY_DIGEST", Activity: "BindPolicyDigest", Stage: "BINDING", Step: "VERIFY_POLICY_DIGEST", ProofChoice: "FOUNDATION", IndicatorClass: "DRIVER"},
	{Ordinal: 4, ID: "BIND_TEST_INVENTORY_DIGEST", Activity: "BindTestInventoryDigest", Stage: "BINDING", Step: "VERIFY_TEST_INVENTORY_DIGEST", ProofChoice: "FOUNDATION", IndicatorClass: "DRIVER"},
	{Ordinal: 5, ID: "BIND_SEMANTIC_CHANGE_GRAPH", Activity: "BindSemanticChangeGraph", Stage: "GRAPH", Step: "VERIFY_SEMANTIC_CHANGE_EDGES", ProofChoice: "COHERENCE", IndicatorClass: "OUTCOME"},
	{Ordinal: 6, ID: "VERIFY_PRIOR_RECEIPTS", Activity: "VerifyImmutablePriorReceipts", Stage: "EVIDENCE", Step: "VERIFY_PRIOR_TEST_RECEIPTS", ProofChoice: "COHERENCE", IndicatorClass: "OUTCOME"},
	{Ordinal: 7, ID: "COMPUTE_INVALIDATION_FRONTIER", Activity: "ComputeMinimalInvalidationFrontier", Stage: "GRAPH", Step: "COMPUTE_MINIMAL_INVALIDATION_FRONTIER", ProofChoice: "COHERENCE", IndicatorClass: "GUARDRAIL"},
	{Ordinal: 8, ID: "CLASSIFY_TEST_ACTIVITIES", Activity: "ClassifyTestActivities", Stage: "TEST", Step: "CLASSIFY_EXECUTED_REUSED_SKIPPED_NOT_OBSERVED", ProofChoice: "COHERENCE", IndicatorClass: "GUARDRAIL"},
	{Ordinal: 9, ID: "VERIFY_COUNTEREXAMPLES", Activity: "VerifyFalseNegativeCounterexamples", Stage: "REGRESSION", Step: "VERIFY_COUNTEREXAMPLE_PRESERVATION", ProofChoice: "REGRESSION", IndicatorClass: "OUTCOME"},
	{Ordinal: 10, ID: "ASSESS_TEST_ECONOMY", Activity: "AssessExactTestEconomy", Stage: "ECONOMY", Step: "COMPARE_EXACT_BEFORE_AFTER_PAIR", ProofChoice: "REGRESSION", IndicatorClass: "OUTCOME"},
	{Ordinal: 11, ID: "ACCOUNT_EXECUTION_STATUSES", Activity: "AccountTestExecutionStatuses", Stage: "OBSERVATION", Step: "ACCOUNT_EXACT_TEST_STATUS_COUNTS", ProofChoice: "REGRESSION", IndicatorClass: "GUARDRAIL"},
	{Ordinal: 12, ID: "EMIT_REPORT_ARTIFACTS", Activity: "EmitTestFrontierReport", Stage: "REPORT", Step: "EMIT_RECEIPT_AND_HUMAN_REPORT", ProofChoice: "REGRESSION", IndicatorClass: "GUARDRAIL"},
}

func EvaluateFixture(fixture Fixture, inputDigest string, source, semanticIR, generatedGo, evaluator, contract ArtifactBinding) (Evaluation, error) {
	if err := ValidateFixture(fixture); err != nil {
		return Evaluation{}, err
	}
	testDecisions, frontierEdges := classifyTests(fixture)
	counts := countTestStatuses(testDecisions)
	evidence := verifyReceipts(fixture)
	economy := assessEconomy(fixture)
	activities := buildActivityDecisions(fixture, testDecisions, counts, evidence, economy)
	activitySummary := summarizeActivities(activities)
	state, reason := aggregateState(activities)
	if economy.State == StateRefuted {
		state = StateRefuted
		reason = "REFUTED: a false-negative counterexample or contradictory economy evidence takes precedence over UNKNOWN and CLOSED."
	}
	plan := Plan{
		Schema: PlanSchema, CaseID: fixture.CaseID, State: state, DecisionReason: reason,
		Activities: activities, Tests: testDecisions, InvalidationFrontier: frontierEdges,
		InvalidatedEdgeCount: len(frontierEdges), ExecutionCounts: counts, Evidence: evidence,
		EvidenceTiming: fixture.EvidenceTiming, Economy: economy, ActivitySummary: activitySummary,
		OutputArtifacts: 3,
	}
	plan.Dossier = renderDossier(plan, fixture)
	receipt := Receipt{
		Schema: ReceiptSchema, CaseID: fixture.CaseID, State: state, DecisionReason: reason,
		Source: source, SemanticIR: semanticIR, GeneratedGo: generatedGo, Evaluator: evaluator, Contract: contract,
		Activities: activitySummary, ExecutionCounts: counts, InvalidatedEdgeCount: len(frontierEdges),
		EvidenceTiming: fixture.EvidenceTiming, EconomyState: economy.State, OutputArtifacts: 3,
		Authority: Authority{RepositoryWrites: 0, LocalTestExecutions: 0, CrossProjectRequiredGates: 0},
	}
	return Evaluation{Plan: plan, Receipt: receipt}, nil
}

func classifyTests(fixture Fixture) ([]TestDecision, []InvalidationFrontierEdge) {
	prior := map[string]PriorTestReceipt{}
	for _, receipt := range fixture.PriorReceipts {
		prior[receipt.TestID] = receipt
	}
	edges := append([]ChangeEdge(nil), fixture.Graph.Edges...)
	sort.Slice(edges, func(i, j int) bool { return edges[i].EdgeID < edges[j].EdgeID })
	decisions := make([]TestDecision, 0, len(fixture.Tests))
	frontierByID := map[string]InvalidationFrontierEdge{}
	for _, test := range fixture.Tests {
		path, affected := minimalPath(fixture.Graph.ChangedUnits, edges, test.TargetNode)
		decision := TestDecision{TestID: test.TestID, OperationID: test.OperationID, Affected: affected, MinimalInvalidationFrontier: append([]string{}, path...)}
		switch {
		case test.Policy == "SKIP":
			decision.Status = "SKIPPED"
			decision.Reason = "POLICY_EXCLUDED_TEST"
		case affected && simulationObserved(test.Simulation):
			decision.Status = "EXECUTED"
			decision.Reason = "SEMANTIC_CHANGE_REACHES_TEST"
			if test.Simulation != nil {
				decision.WallMS = test.Simulation.WallMS
				decision.PeakRSSKiB = test.Simulation.PeakRSSKiB
				decision.ResultDigest = test.Simulation.ResultDigest
			}
		case affected:
			decision.Status = "NOT_OBSERVED"
			decision.Reason = "AFFECTED_TEST_EXECUTION_NOT_OBSERVED"
		case validPriorReceipt(test, prior[test.TestID], fixture.InputBindings):
			decision.Status = "REUSED"
			decision.Reason = "EXACT_IMMUTABLE_PRIOR_RECEIPT_REUSED"
			decision.PriorReceiptID = prior[test.TestID].ReceiptID
		default:
			decision.Status = "NOT_OBSERVED"
			if _, ok := prior[test.TestID]; ok {
				decision.Reason = priorReceiptReason(test, prior[test.TestID], fixture.InputBindings)
			} else {
				decision.Reason = "PRIOR_TEST_RECEIPT_NOT_OBSERVED"
			}
		}
		for _, edgeID := range path {
			for _, edge := range edges {
				if edge.EdgeID == edgeID {
					frontierByID[edge.EdgeID] = InvalidationFrontierEdge{EdgeID: edge.EdgeID, From: edge.From, To: edge.To, Kind: edge.Kind}
				}
			}
		}
		decisions = append(decisions, decision)
	}
	frontier := make([]InvalidationFrontierEdge, 0, len(frontierByID))
	for _, edge := range frontierByID {
		frontier = append(frontier, edge)
	}
	sort.Slice(frontier, func(i, j int) bool { return frontier[i].EdgeID < frontier[j].EdgeID })
	return decisions, frontier
}

func minimalPath(changed []string, edges []ChangeEdge, target string) ([]string, bool) {
	starts := append([]string(nil), changed...)
	sort.Strings(starts)
	type item struct {
		node string
		path []string
	}
	queue := make([]item, 0, len(starts))
	visited := map[string]int{}
	for _, start := range starts {
		if start == target {
			return []string{}, true
		}
		if _, ok := visited[start]; !ok {
			visited[start] = 0
			queue = append(queue, item{node: start})
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range edges {
			if edge.From != current.node {
				continue
			}
			nextPath := append(append([]string{}, current.path...), edge.EdgeID)
			if edge.To == target {
				return nextPath, true
			}
			depth, seen := visited[edge.To]
			if !seen || len(nextPath) < depth {
				visited[edge.To] = len(nextPath)
				queue = append(queue, item{node: edge.To, path: nextPath})
			}
		}
	}
	return nil, false
}

func verifyReceipts(fixture Fixture) []EvidenceResult {
	tests := map[string]TestSpec{}
	for _, test := range fixture.Tests {
		tests[test.TestID] = test
	}
	result := make([]EvidenceResult, 0, len(fixture.PriorReceipts))
	receipts := append([]PriorTestReceipt(nil), fixture.PriorReceipts...)
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].ReceiptID < receipts[j].ReceiptID })
	for _, receipt := range receipts {
		test := tests[receipt.TestID]
		valid := validPriorReceipt(test, receipt, fixture.InputBindings)
		reason := "exact source, toolchain, policy, inventory, PASS result, and immutable receipt match"
		if !valid {
			reason = priorReceiptReason(test, receipt, fixture.InputBindings)
		}
		result = append(result, EvidenceResult{TestID: receipt.TestID, ReceiptID: receipt.ReceiptID, Valid: valid, Reason: reason})
	}
	return result
}

func validPriorReceipt(test TestSpec, receipt PriorTestReceipt, bindings InputBindings) bool {
	if receipt.ReceiptID == "" || receipt.TestID != test.TestID || !receipt.Immutable || receipt.SourceDigest != bindings.SourceDigest || receipt.SourceDigest != test.SourceDigest || receipt.ToolchainDigest != bindings.ToolchainDigest || receipt.PolicyDigest != bindings.PolicyDigest || receipt.TestInventoryDigest != bindings.TestInventoryDigest || receipt.TerminalResult != "PASS" {
		return false
	}
	return true
}

func priorReceiptReason(test TestSpec, receipt PriorTestReceipt, bindings InputBindings) string {
	if receipt.ReceiptID == "" {
		return "PRIOR_TEST_RECEIPT_NOT_OBSERVED"
	}
	if !receipt.Immutable {
		return "PRIOR_RECEIPT_NOT_IMMUTABLE"
	}
	if receipt.SourceDigest != bindings.SourceDigest || receipt.SourceDigest != test.SourceDigest {
		return "PRIOR_RECEIPT_SOURCE_DIGEST_MISMATCH"
	}
	if receipt.ToolchainDigest != bindings.ToolchainDigest {
		return "PRIOR_RECEIPT_TOOLCHAIN_DIGEST_MISMATCH"
	}
	if receipt.PolicyDigest != bindings.PolicyDigest {
		return "PRIOR_RECEIPT_POLICY_DIGEST_MISMATCH"
	}
	if receipt.TestInventoryDigest != bindings.TestInventoryDigest {
		return "PRIOR_RECEIPT_TEST_INVENTORY_DIGEST_MISMATCH"
	}
	if receipt.TerminalResult != "PASS" {
		return "PRIOR_RECEIPT_TERMINAL_RESULT_NOT_PASS"
	}
	return "PRIOR_RECEIPT_BINDING_MISMATCH"
}

func assessEconomy(fixture Fixture) EconomyAssessment {
	if hasFalseNegative(fixture.Counterexamples) {
		return EconomyAssessment{State: StateRefuted, Reason: "FALSE_NEGATIVE_COUNTEREXAMPLE_PRESENT", Comparisons: []MetricComparison{}}
	}
	pair := fixture.PerformancePair
	if pair == nil {
		return economyUnknown(fixture, "EXACT_BEFORE_AFTER_PAIR_NOT_OBSERVED", "DIRECT_MISSING", []string{"performance_pair"})
	}
	if !samePairKey(pair.Before.Key, pair.After.Key) || !pairKeyMatchesInput(pair.Before.Key, fixture) {
		return economyUnknown(fixture, "EXACT_BEFORE_AFTER_PAIR_KEY_MISMATCH", "IMMUTABLE_IDENTITY_MISMATCH", []string{"performance_pair.before", "performance_pair.after"})
	}
	comparisons := make([]MetricComparison, 0, 4)
	appendComparison := func(name string, before, after int64) { comparisons = append(comparisons, MetricComparison{Metric: name, Before: before, After: after, Delta: after - before, Improved: after < before}) }
	appendComparison("build_wall_ms", pair.Before.BuildWallMS, pair.After.BuildWallMS)
	appendComparison("test_wall_ms", pair.Before.TestWallMS, pair.After.TestWallMS)
	appendComparison("conformance_wall_ms", pair.Before.ConformanceWallMS, pair.After.ConformanceWallMS)
	appendComparison("peak_rss_kib", pair.Before.PeakRSSKiB, pair.After.PeakRSSKiB)
	return EconomyAssessment{State: StateClosed, Reason: "EXACT_BEFORE_AFTER_PAIR_VERIFIED", Before: &pair.Before, After: &pair.After, Comparisons: comparisons}
}

func economyUnknown(fixture Fixture, reason, class string, blocked []string) EconomyAssessment {
	unknown := &UnknownDetail{Stage: "ECONOMY", Step: "COMPARE_EXACT_BEFORE_AFTER_PAIR", Reason: reason, UnknownClass: class, NextOperation: "PROVIDE_EXACT_BEFORE_AFTER_PAIR", BlockedBy: append([]string{}, blocked...)}
	return EconomyAssessment{State: StateUnknown, Reason: reason, Unknown: unknown, Comparisons: []MetricComparison{}}
}

func pairKeyMatchesInput(key PairKey, fixture Fixture) bool {
	input := fixture.InputBindings
	return key.ScenarioID == fixture.CaseID && key.SourceDigest == input.SourceDigest && key.ToolchainDigest == input.ToolchainDigest && key.PolicyDigest == input.PolicyDigest && key.TestInventoryDigest == input.TestInventoryDigest
}

func samePairKey(left, right PairKey) bool {
	return left == right
}

func hasFalseNegative(counterexamples []Counterexample) bool {
	for _, counterexample := range counterexamples {
		if counterexample.ExpectedInvalidation && !counterexample.ObservedInvalidation {
			return true
		}
	}
	return false
}

func buildActivityDecisions(fixture Fixture, tests []TestDecision, counts ExecutionCounts, evidence []EvidenceResult, economy EconomyAssessment) []ActivityDecision {
	priorState, priorReason, priorBlocked := priorReceiptActivity(fixture, tests)
	classificationState, classificationReason, classificationBlocked := classificationActivity(tests)
	counterState, counterReason := StateClosed, "NO_FALSE_NEGATIVE_COUNTEREXAMPLE_OBSERVED"
	if hasFalseNegative(fixture.Counterexamples) {
		counterState, counterReason = StateRefuted, "FALSE_NEGATIVE_COUNTEREXAMPLE_PRESENT"
	}
	accountState, accountReason, accountBlocked := accountingActivity(tests, counts)
	activities := make([]ActivityDecision, 0, len(activityDefinitions))
	for _, definition := range activityDefinitions {
		activity := ActivityDecision{Ordinal: definition.Ordinal, ID: definition.ID, Activity: definition.Activity, Stage: definition.Stage, Step: definition.Step, ProofChoice: definition.ProofChoice, IndicatorClass: definition.IndicatorClass, State: StateClosed, Reason: "EXACT_BINDING_OBSERVED"}
		switch definition.ID {
		case "BIND_SEMANTIC_CHANGE_GRAPH":
			activity.Reason = "EXACT_SEMANTIC_CHANGE_EDGES_OBSERVED"
		case "VERIFY_PRIOR_RECEIPTS":
			activity.State, activity.Reason = priorState, priorReason
			if activity.State == StateUnknown { activity.Unknown = unknown(definition, activity.Reason, "PRIOR_RECEIPT_NOT_OBSERVED", "PROVIDE_IMMUTABLE_PRIOR_TEST_RECEIPT", priorBlocked) }
		case "COMPUTE_INVALIDATION_FRONTIER":
			activity.Reason = "MINIMAL_INVALIDATION_FRONTIER_COMPUTED"
		case "CLASSIFY_TEST_ACTIVITIES":
			activity.State, activity.Reason = classificationState, classificationReason
			if activity.State == StateUnknown { activity.Unknown = unknown(definition, activity.Reason, "TEST_EXECUTION_NOT_OBSERVED", "OBSERVE_AFFECTED_TEST_EXECUTION", classificationBlocked) }
		case "VERIFY_COUNTEREXAMPLES":
			activity.State, activity.Reason = counterState, counterReason
		case "ASSESS_TEST_ECONOMY":
			activity.State, activity.Reason, activity.Unknown = economy.State, economy.Reason, economy.Unknown
		case "ACCOUNT_EXECUTION_STATUSES":
			activity.State, activity.Reason = accountState, accountReason
			if activity.State == StateUnknown { activity.Unknown = unknown(definition, activity.Reason, "EXECUTION_METRIC_NOT_OBSERVED", "OBSERVE_EXECUTED_TEST_METRICS", accountBlocked) }
		case "EMIT_REPORT_ARTIFACTS":
			dependencyState, dependencyReason, dependencyBlocked := aggregatePriorActivities(activities)
			activity.State, activity.Reason = dependencyState, dependencyReason
			if activity.State == StateUnknown { activity.Unknown = unknown(definition, activity.Reason, "DEPENDENCY_BLOCKED", "RESOLVE_UNKNOWN_TEST_FRONTIER_ACTIVITIES", dependencyBlocked) }
			if activity.State == StateRefuted { activity.Reason = "REPORT_BLOCKED_BY_REFUTED_TEST_FRONTIER_ACTIVITY" }
		}
		activities = append(activities, activity)
	}
	return activities
}

func priorReceiptActivity(fixture Fixture, tests []TestDecision) (State, string, []string) {
	byID := map[string]TestDecision{}
	for _, test := range tests { byID[test.TestID] = test }
	refuted, unknown := []string{}, []string{}
	receipts := map[string]PriorTestReceipt{}
	for _, receipt := range fixture.PriorReceipts { receipts[receipt.TestID] = receipt }
	for _, test := range fixture.Tests {
		decision := byID[test.TestID]
		if test.Policy == "SKIP" || decision.Affected { continue }
		receipt, ok := receipts[test.TestID]
		if !ok { unknown = append(unknown, test.TestID); continue }
		if !validPriorReceipt(test, receipt, fixture.InputBindings) { refuted = append(refuted, test.TestID) }
	}
	if len(refuted) > 0 { return StateRefuted, "IMMUTABLE_PRIOR_RECEIPT_CONTRADICTION", refuted }
	if len(unknown) > 0 { return StateUnknown, "PRIOR_TEST_RECEIPT_NOT_OBSERVED", unknown }
	return StateClosed, "ALL_PRIOR_RECEIPTS_EXACT_AND_IMMUTABLE", []string{}
}

func classificationActivity(tests []TestDecision) (State, string, []string) {
	blocked := []string{}
	for _, test := range tests { if test.Status == "NOT_OBSERVED" { blocked = append(blocked, test.TestID) } }
	if len(blocked) > 0 { return StateUnknown, "TEST_STATUS_NOT_OBSERVED", blocked }
	return StateClosed, "ALL_TESTS_CLASSIFIED_WITH_EXPLICIT_STATUS", []string{}
}

func accountingActivity(tests []TestDecision, counts ExecutionCounts) (State, string, []string) {
	if counts.Total != counts.Executed+counts.Reused+counts.Skipped+counts.NotObserved { return StateRefuted, "TEST_STATUS_COUNTS_DO_NOT_SUM_TO_TOTAL", []string{} }
	blocked := []string{}
	for _, test := range tests {
		switch test.Status {
		case "EXECUTED":
			if test.WallMS == nil || test.PeakRSSKiB == nil { blocked = append(blocked, test.TestID); continue }
			if *test.WallMS < 0 || *test.PeakRSSKiB <= 0 { return StateRefuted, "INVALID_EXECUTED_TEST_METRICS", []string{} }
		case "REUSED", "SKIPPED", "NOT_OBSERVED":
			if test.WallMS != nil || test.PeakRSSKiB != nil { return StateRefuted, "NON_EXECUTED_STATUS_HAS_CURRENT_METRICS", []string{} }
		default:
			return StateRefuted, "UNRECOGNIZED_TEST_STATUS", []string{}
		}
	}
	if len(blocked) > 0 { return StateUnknown, "EXECUTED_TEST_METRICS_NOT_OBSERVED", blocked }
	return StateClosed, "EXACT_TEST_STATUS_COUNTS_AND_METRIC_POLICY_VERIFIED", []string{}
}

func aggregatePriorActivities(activities []ActivityDecision) (State, string, []string) {
	refuted, unknown := []string{}, []string{}
	for _, activity := range activities {
		if activity.State == StateRefuted { refuted = append(refuted, activity.ID) }
		if activity.State == StateUnknown { unknown = append(unknown, activity.ID) }
	}
	if len(refuted) > 0 { return StateRefuted, "DEPENDENCY_REFUTED", refuted }
	if len(unknown) > 0 { return StateUnknown, "DEPENDENCY_UNKNOWN", unknown }
	return StateClosed, "REPORT_ARTIFACTS_READY", []string{}
}

func unknown(definition ContractCell, reason, class, next string, blocked []string) *UnknownDetail {
	if len(blocked) == 0 { blocked = []string{definition.ID} }
	return &UnknownDetail{Stage: definition.Stage, Step: definition.Step, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: append([]string{}, blocked...)}
}

func countTestStatuses(tests []TestDecision) ExecutionCounts {
	counts := ExecutionCounts{Total: len(tests)}
	for _, test := range tests {
		switch test.Status {
		case "EXECUTED": counts.Executed++
		case "REUSED": counts.Reused++
		case "SKIPPED": counts.Skipped++
		case "NOT_OBSERVED": counts.NotObserved++
		}
	}
	return counts
}

func summarizeActivities(activities []ActivityDecision) ActivitySummary {
	summary := ActivitySummary{Total: len(activities)}
	for _, activity := range activities {
		switch activity.State { case StateClosed: summary.Closed++; case StateUnknown: summary.Unknown++; case StateRefuted: summary.Refuted++ }
	}
	return summary
}

func aggregateState(activities []ActivityDecision) (State, string) {
	for _, activity := range activities { if activity.State == StateRefuted { return StateRefuted, "REFUTED: known contradiction takes precedence over UNKNOWN and CLOSED." } }
	for _, activity := range activities { if activity.State == StateUnknown { return StateUnknown, "UNKNOWN: missing evidence is preserved with its minimal causal frontier." } }
	return StateClosed, "CLOSED: every test status and exact evidence relation is accounted for."
}

func simulationObserved(simulation *Simulation) bool { return simulation != nil && simulation.Observed }
