package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/kimjooyoon/gooo-test-frontier/internal/frontier"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	var code int
	switch os.Args[1] {
	case "compile":
		code = compile(os.Args[2:], os.Stdout, os.Stderr)
	case "evaluate":
		code = evaluate(os.Args[2:], os.Stdout, os.Stderr)
	case "conformance":
		code = conformance(os.Args[2:], os.Stdout, os.Stderr)
	case "version":
		fmt.Fprintln(os.Stdout, "gooo-test-frontier/v0.1.0")
	default:
		usage(os.Stderr)
		code = 2
	}
	os.Exit(code)
}

type compiledMeta struct {
	sourceDigest   string
	contractDigest string
	irRaw          []byte
	generatedGo    []byte
}

func compile(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sourcePath := flags.String("source", "examples/test-frontier.gooo", "Gooo source path")
	contractPath := flags.String("contract", "contracts/test-frontier-denominator-v1.json", "fixed denominator path")
	outputIR := flags.String("output-ir", "", "caller-owned absolute semantic IR output")
	outputGo := flags.String("output-go", "", "caller-owned absolute generated Go output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !absolute(*outputIR) || !absolute(*outputGo) {
		fmt.Fprintln(stderr, "compile requires absolute -output-ir and -output-go paths")
		return 2
	}
	meta, err := compileMeta(*sourcePath, *contractPath)
	if err != nil {
		fmt.Fprintf(stderr, "compile: %v\n", err)
		return 1
	}
	if err := writeOutput(*outputIR, meta.irRaw); err != nil {
		fmt.Fprintf(stderr, "write semantic IR: %v\n", err)
		return 1
	}
	if err := writeOutput(*outputGo, meta.generatedGo); err != nil {
		fmt.Fprintf(stderr, "write generated Go: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "compiled source=%s semantic_ir=%s generated_go=%s\n", *sourcePath, *outputIR, *outputGo)
	return 0
}

func evaluate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("evaluate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	sourcePath := flags.String("source", "examples/test-frontier.gooo", "Gooo source path")
	contractPath := flags.String("contract", "contracts/test-frontier-denominator-v1.json", "fixed denominator path")
	fixturePath := flags.String("fixture", "", "semantic change fixture JSON")
	outputDir := flags.String("output-dir", "", "caller-owned absolute output directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *fixturePath == "" || !absolute(*outputDir) {
		fmt.Fprintln(stderr, "evaluate requires -fixture and absolute -output-dir")
		return 2
	}
	meta, err := compileMeta(*sourcePath, *contractPath)
	if err != nil {
		fmt.Fprintf(stderr, "compile authority chain: %v\n", err)
		return 1
	}
	fixture, raw, err := frontier.LoadFixture(*fixturePath)
	if err != nil {
		fmt.Fprintf(stderr, "read fixture: %v\n", err)
		return 1
	}
	evaluatorPath := filepath.Join(*root, "internal", "frontier", "evaluate.go")
	evaluatorRaw, err := os.ReadFile(evaluatorPath)
	if err != nil {
		fmt.Fprintf(stderr, "read evaluator: %v\n", err)
		return 1
	}
	evaluation, err := frontier.EvaluateFixture(fixture, frontier.DigestBytes(raw),
		frontier.ArtifactBinding{Path: *sourcePath, Digest: meta.sourceDigest},
		frontier.ArtifactBinding{Path: "internal/generated/semantic-ir.json", Digest: frontier.DigestBytes(meta.irRaw)},
		frontier.ArtifactBinding{Path: "internal/generated/semantic.gooo.go", Digest: frontier.DigestBytes(meta.generatedGo)},
		frontier.ArtifactBinding{Path: evaluatorPath, Digest: frontier.DigestBytes(evaluatorRaw)},
		frontier.ArtifactBinding{Path: *contractPath, Digest: meta.contractDigest})
	if err != nil {
		fmt.Fprintf(stderr, "evaluate: %v\n", err)
		return 1
	}
	if err := frontier.WriteEvaluation(*outputDir, evaluation); err != nil {
		fmt.Fprintf(stderr, "write evaluation: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s state=%s output=%s\n", fixture.CaseID, evaluation.Plan.State, *outputDir)
	return 0
}

type conformanceIndex struct {
	Schema           string            `json:"schema"`
	CorpusID         string            `json:"corpus_id"`
	DenominatorID    string            `json:"denominator_id"`
	FixedDenominator int               `json:"fixed_denominator"`
	Cases            []conformanceCase `json:"cases"`
	States           map[string]int    `json:"states"`
}

type conformanceCase struct {
	Ordinal              int            `json:"ordinal"`
	CaseID               string         `json:"case_id"`
	State                frontier.State `json:"state"`
	TotalTests           int            `json:"total_tests"`
	Executed             int            `json:"executed"`
	Reused               int            `json:"reused"`
	Skipped              int            `json:"skipped"`
	NotObserved          int            `json:"not_observed"`
	InvalidatedEdgeCount int            `json:"invalidated_edge_count"`
	EconomyState         frontier.State `json:"economy_state"`
}

func conformance(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	sourcePath := flags.String("source", "examples/test-frontier.gooo", "Gooo source path")
	contractPath := flags.String("contract", "contracts/test-frontier-denominator-v1.json", "fixed denominator path")
	corpusPath := flags.String("corpus", "examples/canonical-corpus.json", "canonical corpus")
	outputDir := flags.String("output-dir", "", "caller-owned absolute output directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !absolute(*outputDir) {
		fmt.Fprintln(stderr, "conformance requires absolute -output-dir")
		return 2
	}
	meta, err := compileMeta(*sourcePath, *contractPath)
	if err != nil {
		fmt.Fprintf(stderr, "compile authority chain: %v\n", err)
		return 1
	}
	corpusRaw, err := os.ReadFile(*corpusPath)
	if err != nil {
		fmt.Fprintf(stderr, "read corpus: %v\n", err)
		return 1
	}
	var corpus frontier.Corpus
	decoder := json.NewDecoder(strings.NewReader(string(corpusRaw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		fmt.Fprintf(stderr, "decode corpus: %v\n", err)
		return 1
	}
	if corpus.Schema != frontier.CorpusSchema || corpus.CorpusID == "" || corpus.DenominatorID != "test-frontier-v1" || corpus.FixedDenominator != 12 || len(corpus.Cases) != 10 {
		fmt.Fprintln(stderr, "canonical corpus header or exact case count is invalid")
		return 1
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "create output: %v\n", err)
		return 1
	}
	index := conformanceIndex{Schema: ProtocolSchema, CorpusID: corpus.CorpusID, DenominatorID: corpus.DenominatorID, FixedDenominator: corpus.FixedDenominator, Cases: make([]conformanceCase, 0, len(corpus.Cases)), States: map[string]int{"CLOSED": 0, "UNKNOWN": 0, "REFUTED": 0}}
	for _, corpusCase := range corpus.Cases {
		fixturePath := filepath.Join(*root, corpusCase.Path)
		fixture, fixtureRaw, loadErr := frontier.LoadFixture(fixturePath)
		if loadErr != nil {
			fmt.Fprintf(stderr, "load %s: %v\n", corpusCase.CaseID, loadErr)
			return 1
		}
		if fixture.CaseID != corpusCase.CaseID || fixture.Expected.State != corpusCase.State {
			fmt.Fprintf(stderr, "%s: corpus binding mismatch\n", corpusCase.CaseID)
			return 1
		}
		evaluatorPath := filepath.Join(*root, "internal", "frontier", "evaluate.go")
		evaluatorRaw, readErr := os.ReadFile(evaluatorPath)
		if readErr != nil {
			fmt.Fprintf(stderr, "read evaluator: %v\n", readErr)
			return 1
		}
		evaluation, evalErr := frontier.EvaluateFixture(fixture, frontier.DigestBytes(fixtureRaw),
			frontier.ArtifactBinding{Path: *sourcePath, Digest: meta.sourceDigest},
			frontier.ArtifactBinding{Path: "internal/generated/semantic-ir.json", Digest: frontier.DigestBytes(meta.irRaw)},
			frontier.ArtifactBinding{Path: "internal/generated/semantic.gooo.go", Digest: frontier.DigestBytes(meta.generatedGo)},
			frontier.ArtifactBinding{Path: evaluatorPath, Digest: frontier.DigestBytes(evaluatorRaw)},
			frontier.ArtifactBinding{Path: *contractPath, Digest: meta.contractDigest})
		if evalErr != nil {
			fmt.Fprintf(stderr, "evaluate %s: %v\n", corpusCase.CaseID, evalErr)
			return 1
		}
		if err := assertExpectations(fixture, evaluation); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", corpusCase.CaseID, err)
			return 1
		}
		caseDir := filepath.Join(*outputDir, corpusCase.CaseID)
		if err := frontier.WriteEvaluation(caseDir, evaluation); err != nil {
			fmt.Fprintf(stderr, "write %s: %v\n", corpusCase.CaseID, err)
			return 1
		}
		counts := evaluation.Plan.ExecutionCounts
		index.Cases = append(index.Cases, conformanceCase{Ordinal: corpusCase.Ordinal, CaseID: corpusCase.CaseID, State: evaluation.Plan.State, TotalTests: counts.Total, Executed: counts.Executed, Reused: counts.Reused, Skipped: counts.Skipped, NotObserved: counts.NotObserved, InvalidatedEdgeCount: evaluation.Plan.InvalidatedEdgeCount, EconomyState: evaluation.Plan.Economy.State})
		index.States[string(evaluation.Plan.State)]++
		fmt.Fprintf(stdout, "%s state=%s tests=%d/%d/%d/%d\n", corpusCase.CaseID, evaluation.Plan.State, counts.Executed, counts.Reused, counts.Skipped, counts.NotObserved)
	}
	return writeJSON(filepath.Join(*outputDir, "conformance-index.json"), index, stderr)
}

const ProtocolSchema = "gooo/test-frontier/conformance/v1"

func assertExpectations(fixture frontier.Fixture, evaluation frontier.Evaluation) error {
	expected := fixture.Expected
	actual := evaluation.Plan
	if actual.State != expected.State {
		return fmt.Errorf("expected state %s, got %s", expected.State, actual.State)
	}
	if !reflect.DeepEqual(expected.TestCounts, actual.ExecutionCounts) {
		return fmt.Errorf("test counts differ: expected %+v, got %+v", expected.TestCounts, actual.ExecutionCounts)
	}
	if actual.InvalidatedEdgeCount != expected.InvalidatedEdgeCount {
		return fmt.Errorf("invalidated edge count differs: expected %d, got %d", expected.InvalidatedEdgeCount, actual.InvalidatedEdgeCount)
	}
	if actual.Economy.State != expected.EconomyState {
		return fmt.Errorf("economy state differs: expected %s, got %s", expected.EconomyState, actual.Economy.State)
	}
	if actual.ExecutionCounts.Total != actual.ExecutionCounts.Executed+actual.ExecutionCounts.Reused+actual.ExecutionCounts.Skipped+actual.ExecutionCounts.NotObserved {
		return fmt.Errorf("test status counts do not sum to total")
	}
	if actual.ActivitySummary.Total != 12 {
		return fmt.Errorf("activity denominator is %d", actual.ActivitySummary.Total)
	}
	if fixture.ExternalAuthority != nil {
		if actual.ExternalAuthority == nil || actual.ExternalAuthority.PlatformImmutable != fixture.ExternalAuthority.PlatformImmutable {
			return fmt.Errorf("external authority evidence was not preserved")
		}
		if fixture.ExternalAuthority.ExpectedPlatformImmutable && fixture.ExternalAuthority.SelfAssertedImmutable && !fixture.ExternalAuthority.PlatformImmutable {
			if actual.State != frontier.StateRefuted || actual.Activities[8].State != frontier.StateRefuted || actual.Activities[8].Reason != "SELF_ASSERTED_IMMUTABILITY_CONTRADICTED_BY_PLATFORM" {
				return fmt.Errorf("external platform contradiction did not remain REFUTED")
			}
		}
	}
	for _, activity := range actual.Activities {
		if activity.State == frontier.StateUnknown && (activity.Unknown == nil || activity.Unknown.Stage == "" || activity.Unknown.Step == "" || activity.Unknown.Reason == "" || activity.Unknown.UnknownClass == "" || activity.Unknown.NextOperation == "" || len(activity.Unknown.BlockedBy) == 0) {
			return fmt.Errorf("activity %s has incomplete UNKNOWN fields", activity.ID)
		}
	}
	return nil
}

func compileMeta(sourcePath, contractPath string) (compiledMeta, error) {
	sourceDigest, sourceRaw, err := frontier.DigestFile(sourcePath)
	if err != nil {
		return compiledMeta{}, err
	}
	contract, contractRaw, err := frontier.LoadContract(contractPath)
	if err != nil {
		return compiledMeta{}, err
	}
	ir, err := frontier.CompileSource(sourcePath, sourceRaw, contract, frontier.DigestBytes(contractRaw))
	if err != nil {
		return compiledMeta{}, err
	}
	irRaw, err := frontier.SemanticIRBytes(ir)
	if err != nil {
		return compiledMeta{}, err
	}
	return compiledMeta{sourceDigest: sourceDigest, contractDigest: frontier.DigestBytes(contractRaw), irRaw: irRaw, generatedGo: frontier.GenerateGo(ir, frontier.DigestBytes(irRaw))}, nil
}

func writeOutput(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func writeJSON(path string, value any, stderr io.Writer) int {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode JSON: %v\n", err)
		return 1
	}
	if err := writeOutput(path, append(raw, '\n')); err != nil {
		fmt.Fprintf(stderr, "write JSON: %v\n", err)
		return 1
	}
	return 0
}

func absolute(path string) bool { return path != "" && filepath.IsAbs(path) }

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: gooo-test-frontier <compile|evaluate|conformance|version>")
}
