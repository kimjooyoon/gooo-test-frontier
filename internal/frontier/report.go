package frontier

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func WriteEvaluation(outputDir string, evaluation Evaluation) error {
	if !filepath.IsAbs(outputDir) {
		return errors.New("output directory must be an absolute caller-owned path")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	planRaw, err := json.MarshalIndent(evaluation.Plan, "", "  ")
	if err != nil {
		return err
	}
	receiptRaw, err := json.MarshalIndent(evaluation.Receipt, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "plan.json"), append(planRaw, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "receipt.json"), append(receiptRaw, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "human-report.md"), []byte(evaluation.Plan.Dossier), 0o644)
}

func renderDossier(plan Plan, fixture Fixture) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Test frontier dossier: `%s`\n\n", plan.CaseID)
	fmt.Fprintf(&builder, "- Decision: **%s**\n- Reason: %s\n- Fixed activities: `%d` (1:1)\n- Tests: total=`%d`, executed=`%d`, reused=`%d`, skipped=`%d`, not_observed=`%d`\n- Invalidated edges: `%d`\n- Evidence lookup / verification: `%d ms` / `%d ms`\n- Output artifacts: `%d`\n\n", plan.State, plan.DecisionReason, plan.ActivitySummary.Total, plan.ExecutionCounts.Total, plan.ExecutionCounts.Executed, plan.ExecutionCounts.Reused, plan.ExecutionCounts.Skipped, plan.ExecutionCounts.NotObserved, plan.InvalidatedEdgeCount, plan.EvidenceTiming.LookupMS, plan.EvidenceTiming.VerificationMS, plan.OutputArtifacts)
	builder.WriteString("## Test activity decisions\n\n")
	for _, test := range plan.Tests {
		frontier := "none"
		if len(test.MinimalInvalidationFrontier) > 0 {
			frontier = "`" + strings.Join(test.MinimalInvalidationFrontier, "`, `") + "`"
		}
		fmt.Fprintf(&builder, "- `%s` → **%s**: %s; affected=%t; minimal frontier=%s.\n", test.TestID, test.Status, test.Reason, test.Affected, frontier)
	}
	builder.WriteString("\n## Minimal invalidation frontier\n\n")
	if len(plan.InvalidationFrontier) == 0 {
		builder.WriteString("No changed semantic unit reaches a test target.\n\n")
	}
	for _, edge := range plan.InvalidationFrontier {
		fmt.Fprintf(&builder, "- `%s`: `%s` → `%s` (%s)\n", edge.EdgeID, edge.From, edge.To, edge.Kind)
	}
	builder.WriteString("\n## Evidence\n\n")
	if len(plan.Evidence) == 0 {
		builder.WriteString("No prior receipt was supplied.\n\n")
	}
	for _, evidence := range plan.Evidence {
		fmt.Fprintf(&builder, "- `%s` / `%s`: valid=%t — %s.\n", evidence.TestID, evidence.ReceiptID, evidence.Valid, evidence.Reason)
	}
	if plan.ExternalAuthority != nil {
		authority := plan.ExternalAuthority
		builder.WriteString("\n## External authority\n\n")
		fmt.Fprintf(&builder, "- Provider: `%s`; repository: `%s`; endpoint: `%s`; release: `%s` (ID `%d`).\n", authority.Provider, authority.Repository, authority.Endpoint, authority.ReleaseTag, authority.ReleaseID)
		fmt.Fprintf(&builder, "- Self-asserted immutable=`%t`; platform immutable=`%t`; required platform immutable=`%t`.\n", authority.SelfAssertedImmutable, authority.PlatformImmutable, authority.ExpectedPlatformImmutable)
		fmt.Fprintf(&builder, "- Authority result: **%s** — %s.\n", StateRefuted, authority.Reason)
	}
	builder.WriteString("\n## Exact test economy\n\n")
	fmt.Fprintf(&builder, "Economy state: **%s** — %s.\n\n", plan.Economy.State, plan.Economy.Reason)
	if len(plan.Economy.Comparisons) > 0 {
		builder.WriteString("| Metric | Before | After | Delta | Improved |\n|---|---:|---:|---:|---|\n")
		for _, comparison := range plan.Economy.Comparisons {
			fmt.Fprintf(&builder, "| %s | %d | %d | %d | %t |\n", comparison.Metric, comparison.Before, comparison.After, comparison.Delta, comparison.Improved)
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Meta activity resolution\n\n")
	for _, activity := range plan.Activities {
		fmt.Fprintf(&builder, "- `%02d %s` (%s/%s, proof=%s, indicator=%s): **%s** — %s.\n", activity.Ordinal, activity.ID, activity.Stage, activity.Step, activity.ProofChoice, activity.IndicatorClass, activity.State, activity.Reason)
		if activity.Unknown != nil {
			fmt.Fprintf(&builder, "  UNKNOWN: stage=%s; step=%s; reason=%s; unknown_class=%s; next_operation=%s; blocked_by=`%s`.\n", activity.Unknown.Stage, activity.Unknown.Step, activity.Unknown.Reason, activity.Unknown.UnknownClass, activity.Unknown.NextOperation, strings.Join(activity.Unknown.BlockedBy, "`, `"))
		}
	}
	builder.WriteString("\nThe evaluator is fixture simulation only. It does not modify a target repository or execute target tests. Product authority is fixed at repository_writes=0, local_test_executions=0, cross_project_required_gates=0.\n")
	_ = fixture
	return builder.String()
}
