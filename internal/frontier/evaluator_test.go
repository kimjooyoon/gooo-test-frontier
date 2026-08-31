package frontier

import (
	"path/filepath"
	"testing"
)

func TestNormalCaseClassifiesEveryTestAndKeepsMinimalPath(t *testing.T) {
	fixture, raw, err := LoadFixture(filepath.Join("..", "..", "fixtures", "cases", "normal-alpha.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateFixture(fixture, DigestBytes(raw), ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{})
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Plan.State != StateClosed {
		t.Fatalf("state = %s, want %s", evaluation.Plan.State, StateClosed)
	}
	if evaluation.Plan.ExecutionCounts != (ExecutionCounts{Total: 4, Executed: 1, Reused: 2, Skipped: 1, NotObserved: 0}) {
		t.Fatalf("counts = %+v", evaluation.Plan.ExecutionCounts)
	}
	if evaluation.Plan.InvalidatedEdgeCount != 2 || len(evaluation.Plan.Tests[0].MinimalInvalidationFrontier) != 2 {
		t.Fatalf("frontier = %+v", evaluation.Plan)
	}
	if evaluation.Plan.Tests[1].Status != "REUSED" || evaluation.Plan.Tests[1].WallMS != nil {
		t.Fatalf("reuse carried current timing: %+v", evaluation.Plan.Tests[1])
	}
}

func TestCacheHitAloneCannotReuse(t *testing.T) {
	fixture, raw, err := LoadFixture(filepath.Join("..", "..", "fixtures", "cases", "unknown-cache-hit-only.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateFixture(fixture, DigestBytes(raw), ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{})
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Plan.State != StateUnknown {
		t.Fatalf("state = %s, want UNKNOWN", evaluation.Plan.State)
	}
	if evaluation.Plan.ExecutionCounts.Reused != 0 || evaluation.Plan.ExecutionCounts.NotObserved != 2 {
		t.Fatalf("cache hit was treated as evidence: %+v", evaluation.Plan.ExecutionCounts)
	}
}

func TestFalseNegativePrecedesUnknownAndClosesEconomyAsRefuted(t *testing.T) {
	fixture, raw, err := LoadFixture(filepath.Join("..", "..", "fixtures", "cases", "refuted-false-negative.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateFixture(fixture, DigestBytes(raw), ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{}, ArtifactBinding{})
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Plan.State != StateRefuted || evaluation.Plan.Economy.State != StateRefuted {
		t.Fatalf("false negative was not dominant: %+v", evaluation.Plan)
	}
	if evaluation.Plan.ActivitySummary.Refuted < 3 {
		t.Fatalf("refutation was not preserved in dependent activities: %+v", evaluation.Plan.ActivitySummary)
	}
}

func TestNonExecutedStatusesHaveNoCurrentMetrics(t *testing.T) {
	err := ValidateFixture(Fixture{Schema: ProtocolSchema + "/fixture/v1"})
	if err == nil {
		t.Fatal("incomplete fixture unexpectedly validated")
	}
}
