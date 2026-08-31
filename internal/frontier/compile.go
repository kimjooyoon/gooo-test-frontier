package frontier

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func LoadContract(path string) (Contract, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, nil, err
	}
	var contract Contract
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return Contract{}, nil, err
	}
	if err := ValidateContract(contract); err != nil {
		return Contract{}, nil, err
	}
	return contract, raw, nil
}

func ValidateContract(contract Contract) error {
	if contract.Schema != ProtocolSchema+"/denominator/v1" || contract.DenominatorID == "" || contract.FixedDenominator != 12 || contract.MetaActivityCount != 12 || len(contract.Cells) != 12 {
		return errors.New("INVALID_FIXED_DENOMINATOR")
	}
	if contract.ExternalAuthority.Provider != "github" || contract.ExternalAuthority.Repository == "" || contract.ExternalAuthority.ReleaseImmutabilityEndpoint == "" || contract.ExternalAuthority.ReleaseField != "immutable" || !contract.ExternalAuthority.Required || !contract.ExternalAuthority.SelfAssertedIsInsufficient || contract.ExternalAuthority.ContradictionReason != "SELF_ASSERTED_IMMUTABILITY_CONTRADICTED_BY_PLATFORM" {
		return errors.New("INVALID_EXTERNAL_AUTHORITY_CONTRACT")
	}
	if len(contract.StatePrecedence) != 3 || contract.StatePrecedence[0] != StateRefuted || contract.StatePrecedence[1] != StateUnknown || contract.StatePrecedence[2] != StateClosed {
		return errors.New("INVALID_STATE_PRECEDENCE")
	}
	proofs := map[string]int{}
	indicators := map[string]int{}
	seenIDs := map[string]bool{}
	seenActivities := map[string]bool{}
	for index, cell := range contract.Cells {
		if cell.Ordinal != index+1 || cell.ID == "" || cell.Activity == "" || cell.Stage == "" || cell.Step == "" || !validProofChoice(ProofChoice(cell.ProofChoice)) || !validIndicator(cell.IndicatorClass) || seenIDs[cell.ID] || seenActivities[cell.Activity] {
			return fmt.Errorf("INVALID_CONTRACT_CELL_%d", index+1)
		}
		seenIDs[cell.ID] = true
		seenActivities[cell.Activity] = true
		proofs[cell.ProofChoice]++
		indicators[cell.IndicatorClass]++
	}
	for _, choice := range []string{"FOUNDATION", "COHERENCE", "REGRESSION"} {
		if proofs[choice] != 4 || contract.ProofTotals[choice] != 4 {
			return fmt.Errorf("INVALID_PROOF_DISTRIBUTION_%s", choice)
		}
	}
	for _, class := range []string{"DRIVER", "OUTCOME", "GUARDRAIL"} {
		if indicators[class] != 4 || contract.IndicatorTotals[class] != 4 {
			return fmt.Errorf("INVALID_INDICATOR_DISTRIBUTION_%s", class)
		}
	}
	return nil
}

func CompileSource(sourcePath string, source []byte, contract Contract, contractDigest string) (SemanticIR, error) {
	if err := ValidateContract(contract); err != nil {
		return SemanticIR{}, err
	}
	lines := strings.Split(string(source), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != `@gooo schema="gooo/test-frontier/v1"` {
		return SemanticIR{}, errors.New("GOOO_SOURCE_SCHEMA_HEADER_MISSING")
	}
	activities := make([]SourceActivity, 0, len(contract.Cells))
	for lineNumber, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@gooo ") {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) != 8 || fields[0] != "activity" {
			return SemanticIR{}, fmt.Errorf("source line %d: INVALID_ACTIVITY_RECORD", lineNumber+1)
		}
		ordinal, err := strconv.Atoi(fields[1])
		if err != nil {
			return SemanticIR{}, fmt.Errorf("source line %d: INVALID_ORDINAL", lineNumber+1)
		}
		activity := SourceActivity{Ordinal: ordinal, ID: fields[2], Activity: fields[3], Stage: fields[4], Step: fields[5], ProofChoice: fields[6], IndicatorClass: fields[7], SourceLine: lineNumber + 1}
		if activity.ID == "" || activity.Activity == "" || activity.Stage == "" || activity.Step == "" || !validProofChoice(ProofChoice(activity.ProofChoice)) || !validIndicator(activity.IndicatorClass) {
			return SemanticIR{}, fmt.Errorf("source line %d: INCOMPLETE_ACTIVITY_METADATA", lineNumber+1)
		}
		activities = append(activities, activity)
	}
	if len(activities) != contract.MetaActivityCount {
		return SemanticIR{}, errors.New("GOOO_SOURCE_DENOMINATOR_CARDINALITY_MISMATCH")
	}
	for index, activity := range activities {
		cell := contract.Cells[index]
		if activity.Ordinal != cell.Ordinal || activity.ID != cell.ID || activity.Activity != cell.Activity || activity.Stage != cell.Stage || activity.Step != cell.Step || activity.ProofChoice != cell.ProofChoice || activity.IndicatorClass != cell.IndicatorClass {
			return SemanticIR{}, fmt.Errorf("GOOO_ACTIVITY_BINDING_MISMATCH_%d", cell.Ordinal)
		}
	}
	return SemanticIR{Schema: IRSchema, SourcePath: filepath.ToSlash(sourcePath), SourceDigest: DigestBytes(source), ContractDigest: contractDigest, MetaActivityCount: len(activities), Activities: activities}, nil
}

func SemanticIRBytes(ir SemanticIR) ([]byte, error) {
	raw, err := json.MarshalIndent(ir, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func GenerateGo(ir SemanticIR, semanticDigest string) []byte {
	var builder strings.Builder
	builder.WriteString("// Code generated by gooo. DO NOT EDIT.\n\n")
	builder.WriteString("package generated\n\n")
	builder.WriteString("const ProtocolSchema = \"gooo/test-frontier/protocol/v1\"\n")
	fmt.Fprintf(&builder, "const SourcePath = %q\n", ir.SourcePath)
	fmt.Fprintf(&builder, "const SourceDigest = %q\n", ir.SourceDigest)
	fmt.Fprintf(&builder, "const SemanticIRPath = %q\n", "internal/generated/semantic-ir.json")
	fmt.Fprintf(&builder, "const SemanticIRDigest = %q\n", semanticDigest)
	fmt.Fprintf(&builder, "const ContractPath = %q\n", "contracts/test-frontier-denominator-v1.json")
	fmt.Fprintf(&builder, "const ContractDigest = %q\n", ir.ContractDigest)
	fmt.Fprintf(&builder, "const MetaActivityCount = %d\n\n", ir.MetaActivityCount)
	builder.WriteString("type Activity struct {\n\tOrdinal        int\n\tID             string\n\tActivity       string\n\tStage          string\n\tStep           string\n\tProofChoice    string\n\tIndicatorClass string\n}\n\n")
	builder.WriteString("var Activities = []Activity{\n")
	for _, activity := range ir.Activities {
		fmt.Fprintf(&builder, "\t{Ordinal: %d, ID: %q, Activity: %q, Stage: %q, Step: %q, ProofChoice: %q, IndicatorClass: %q},\n", activity.Ordinal, activity.ID, activity.Activity, activity.Stage, activity.Step, activity.ProofChoice, activity.IndicatorClass)
	}
	builder.WriteString("}\n")
	return []byte(builder.String())
}

func LoadFixture(path string) (Fixture, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, nil, err
	}
	var fixture Fixture
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return Fixture{}, nil, err
	}
	if err := ValidateFixture(fixture); err != nil {
		return Fixture{}, nil, err
	}
	return fixture, raw, nil
}

func ValidateFixture(fixture Fixture) error {
	if fixture.Schema != ProtocolSchema+"/fixture/v1" || fixture.CaseID == "" || fixture.Graph.Schema != ProtocolSchema+"/semantic-change-graph/v1" || fixture.Graph.GraphID == "" || len(fixture.Graph.ChangedUnits) == 0 || len(fixture.Tests) == 0 {
		return errors.New("INVALID_FIXTURE_HEADER")
	}
	for name, value := range map[string]string{
		"source_digest":                fixture.InputBindings.SourceDigest,
		"toolchain_digest":             fixture.InputBindings.ToolchainDigest,
		"policy_digest":                fixture.InputBindings.PolicyDigest,
		"test_inventory_digest":        fixture.InputBindings.TestInventoryDigest,
		"semantic_change_graph_digest": fixture.InputBindings.SemanticChangeGraphDigest,
	} {
		if err := ValidateDigest(value); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	seenTests := map[string]bool{}
	for _, test := range fixture.Tests {
		if test.TestID == "" || test.OperationID == "" || test.TargetNode == "" || test.SourceDigest == "" || seenTests[test.TestID] || (test.Policy != "REQUIRED" && test.Policy != "OPTIONAL" && test.Policy != "SKIP") {
			return fmt.Errorf("INVALID_TEST_%s", test.TestID)
		}
		if test.SourceDigest != fixture.InputBindings.SourceDigest {
			return fmt.Errorf("TEST_SOURCE_DIGEST_BINDING_MISMATCH_%s", test.TestID)
		}
		if err := ValidateDigest(test.SourceDigest); err != nil {
			return fmt.Errorf("test %s: %w", test.TestID, err)
		}
		if test.Simulation != nil {
			if !test.Simulation.Observed && (test.Simulation.WallMS != nil || test.Simulation.PeakRSSKiB != nil) {
				return fmt.Errorf("TEST_SIMULATION_METRICS_WITHOUT_OBSERVATION_%s", test.TestID)
			}
			if test.Simulation.Observed && (test.Simulation.WallMS == nil || test.Simulation.PeakRSSKiB == nil) {
				return fmt.Errorf("TEST_SIMULATION_METRICS_NOT_OBSERVED_%s", test.TestID)
			}
			if test.Simulation.ResultDigest != "" {
				if err := ValidateDigest(test.Simulation.ResultDigest); err != nil {
					return fmt.Errorf("test %s simulation: %w", test.TestID, err)
				}
			}
		}
		seenTests[test.TestID] = true
	}
	seenEdges := map[string]bool{}
	for _, edge := range fixture.Graph.Edges {
		if edge.EdgeID == "" || edge.From == "" || edge.To == "" || edge.Kind == "" || seenEdges[edge.EdgeID] {
			return errors.New("INVALID_SEMANTIC_CHANGE_EDGE")
		}
		seenEdges[edge.EdgeID] = true
	}
	seenReceipts := map[string]bool{}
	seenReceiptTests := map[string]bool{}
	for _, receipt := range fixture.PriorReceipts {
		if receipt.ReceiptID == "" || receipt.TestID == "" || seenReceipts[receipt.ReceiptID] || seenReceiptTests[receipt.TestID] {
			return errors.New("INVALID_PRIOR_RECEIPT")
		}
		if !seenTests[receipt.TestID] {
			return fmt.Errorf("PRIOR_RECEIPT_TEST_NOT_IN_INVENTORY_%s", receipt.ReceiptID)
		}
		if err := ValidateDigest(receipt.SourceDigest); err != nil {
			return fmt.Errorf("receipt %s: %w", receipt.ReceiptID, err)
		}
		if err := ValidateDigest(receipt.ToolchainDigest); err != nil {
			return fmt.Errorf("receipt %s: %w", receipt.ReceiptID, err)
		}
		if err := ValidateDigest(receipt.PolicyDigest); err != nil {
			return fmt.Errorf("receipt %s: %w", receipt.ReceiptID, err)
		}
		if err := ValidateDigest(receipt.TestInventoryDigest); err != nil {
			return fmt.Errorf("receipt %s: %w", receipt.ReceiptID, err)
		}
		if err := ValidateDigest(receipt.ResultDigest); err != nil {
			return fmt.Errorf("receipt %s: %w", receipt.ReceiptID, err)
		}
		if receipt.WallMS < 0 || receipt.PeakRSSKiB <= 0 {
			return fmt.Errorf("INVALID_PRIOR_RECEIPT_METRICS_%s", receipt.ReceiptID)
		}
		seenReceipts[receipt.ReceiptID] = true
		seenReceiptTests[receipt.TestID] = true
	}
	for _, counterexample := range fixture.Counterexamples {
		if counterexample.CounterexampleID == "" || counterexample.TestID == "" || !seenTests[counterexample.TestID] {
			return errors.New("INVALID_COUNTEREXAMPLE")
		}
	}
	if fixture.EvidenceTiming.LookupMS < 0 || fixture.EvidenceTiming.VerificationMS < 0 {
		return errors.New("INVALID_EVIDENCE_TIMING")
	}
	if fixture.PerformancePair != nil {
		for _, snapshot := range []PerformanceSnapshot{fixture.PerformancePair.Before, fixture.PerformancePair.After} {
			if err := validatePairKey(snapshot.Key); err != nil {
				return err
			}
			if snapshot.BuildWallMS < 0 || snapshot.TestWallMS < 0 || snapshot.ConformanceWallMS < 0 || snapshot.PeakRSSKiB <= 0 {
				return errors.New("INVALID_PERFORMANCE_SNAPSHOT_METRICS")
			}
		}
	}
	if authority := fixture.ExternalAuthority; authority != nil {
		if authority.Provider != "github" || authority.Repository == "" || authority.Endpoint == "" || authority.ReleaseID <= 0 || authority.ReleaseTag == "" || authority.Reason == "" {
			return errors.New("INVALID_EXTERNAL_AUTHORITY_EVIDENCE")
		}
		if authority.ExpectedPlatformImmutable && authority.SelfAssertedImmutable && !authority.PlatformImmutable && authority.Reason != "SELF_ASSERTED_IMMUTABILITY_CONTRADICTED_BY_PLATFORM" {
			return errors.New("INVALID_EXTERNAL_AUTHORITY_CONTRADICTION_REASON")
		}
	}
	if fixture.Expected.State != StateClosed && fixture.Expected.State != StateUnknown && fixture.Expected.State != StateRefuted {
		return errors.New("INVALID_EXPECTED_STATE")
	}
	if fixture.Expected.EconomyState != StateClosed && fixture.Expected.EconomyState != StateUnknown && fixture.Expected.EconomyState != StateRefuted {
		return errors.New("INVALID_EXPECTED_ECONOMY_STATE")
	}
	return nil
}

func validatePairKey(key PairKey) error {
	if key.ScenarioID == "" {
		return errors.New("INVALID_PERFORMANCE_PAIR_SCENARIO")
	}
	for name, value := range map[string]string{"source": key.SourceDigest, "toolchain": key.ToolchainDigest, "policy": key.PolicyDigest, "inventory": key.TestInventoryDigest} {
		if err := ValidateDigest(value); err != nil {
			return fmt.Errorf("pair %s: %w", name, err)
		}
	}
	return nil
}

func validProofChoice(choice ProofChoice) bool {
	return choice == ProofFoundation || choice == ProofCoherence || choice == ProofRegression
}

func validIndicator(value string) bool {
	return value == "DRIVER" || value == "OUTCOME" || value == "GUARDRAIL"
}
