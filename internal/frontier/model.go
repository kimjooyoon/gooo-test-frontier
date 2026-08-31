package frontier

const (
	ProtocolSchema = "gooo/test-frontier/protocol/v1"
	SourceSchema   = "gooo/test-frontier/v1"
	IRSchema       = "gooo/test-frontier/ir/v1"
	PlanSchema     = "gooo/test-frontier/plan/v1"
	ReceiptSchema  = "gooo/test-frontier/receipt/v1"
	CorpusSchema   = "gooo/test-frontier/corpus/v1"

	StateClosed  State = "CLOSED"
	StateUnknown State = "UNKNOWN"
	StateRefuted State = "REFUTED"

	ProofFoundation ProofChoice = "FOUNDATION"
	ProofCoherence  ProofChoice = "COHERENCE"
	ProofRegression ProofChoice = "REGRESSION"
)

var StatePrecedence = []State{StateRefuted, StateUnknown, StateClosed}

type State string
type ProofChoice string

type Contract struct {
	Schema            string         `json:"schema"`
	DenominatorID     string         `json:"denominator_id"`
	FixedDenominator  int            `json:"fixed_denominator"`
	MetaActivityCount int            `json:"meta_activity_count"`
	StatePrecedence   []State        `json:"state_precedence"`
	ProofTotals       map[string]int `json:"proof_totals"`
	IndicatorTotals   map[string]int `json:"indicator_totals"`
	Cells             []ContractCell `json:"cells"`
}

type ContractCell struct {
	Ordinal         int    `json:"ordinal"`
	ID              string `json:"id"`
	Activity        string `json:"activity"`
	Stage           string `json:"stage"`
	Step            string `json:"step"`
	ProofChoice     string `json:"proof_choice"`
	IndicatorClass  string `json:"indicator_class"`
}

type SourceActivity struct {
	Ordinal        int    `json:"ordinal"`
	ID             string `json:"id"`
	Activity       string `json:"activity"`
	Stage          string `json:"stage"`
	Step           string `json:"step"`
	ProofChoice    string `json:"proof_choice"`
	IndicatorClass string `json:"indicator_class"`
	SourceLine     int    `json:"source_line"`
}

type SemanticIR struct {
	Schema            string           `json:"schema"`
	SourcePath        string           `json:"source_path"`
	SourceDigest      string           `json:"source_digest"`
	ContractDigest    string           `json:"contract_digest"`
	MetaActivityCount int              `json:"meta_activity_count"`
	Activities        []SourceActivity `json:"activities"`
}

type InputBindings struct {
	SourceDigest              string `json:"source_digest"`
	ToolchainDigest           string `json:"toolchain_digest"`
	PolicyDigest              string `json:"policy_digest"`
	TestInventoryDigest      string `json:"test_inventory_digest"`
	SemanticChangeGraphDigest string `json:"semantic_change_graph_digest"`
	CacheHit                  bool   `json:"cache_hit"`
}

type SemanticChangeGraph struct {
	Schema       string       `json:"schema"`
	GraphID      string       `json:"graph_id"`
	ChangedUnits []string     `json:"changed_units"`
	Edges        []ChangeEdge `json:"edges"`
}

type ChangeEdge struct {
	EdgeID string `json:"edge_id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind"`
}

type TestSpec struct {
	TestID       string      `json:"test_id"`
	OperationID  string      `json:"operation_id"`
	TargetNode   string      `json:"target_node"`
	SourceDigest string      `json:"source_digest"`
	Policy       string      `json:"policy"`
	Simulation   *Simulation `json:"simulation,omitempty"`
}

type Simulation struct {
	Observed     bool    `json:"observed"`
	WallMS       *int64  `json:"wall_ms,omitempty"`
	PeakRSSKiB   *int64  `json:"peak_rss_kib,omitempty"`
	Result       string  `json:"result,omitempty"`
	ResultDigest string  `json:"result_digest,omitempty"`
}

type PriorTestReceipt struct {
	ReceiptID            string `json:"receipt_id"`
	TestID               string `json:"test_id"`
	SourceDigest         string `json:"source_digest"`
	ToolchainDigest      string `json:"toolchain_digest"`
	PolicyDigest         string `json:"policy_digest"`
	TestInventoryDigest  string `json:"test_inventory_digest"`
	TerminalResult       string `json:"terminal_result"`
	ResultDigest         string `json:"result_digest"`
	WallMS               int64  `json:"wall_ms"`
	PeakRSSKiB           int64  `json:"peak_rss_kib"`
	Immutable            bool   `json:"immutable"`
}

type Counterexample struct {
	CounterexampleID   string `json:"counterexample_id"`
	TestID             string `json:"test_id"`
	ExpectedInvalidation bool `json:"expected_invalidation"`
	ObservedInvalidation bool `json:"observed_invalidation"`
}

type PairKey struct {
	ScenarioID          string `json:"scenario_id"`
	SourceDigest        string `json:"source_digest"`
	ToolchainDigest     string `json:"toolchain_digest"`
	PolicyDigest        string `json:"policy_digest"`
	TestInventoryDigest string `json:"test_inventory_digest"`
}

type PerformanceSnapshot struct {
	Key              PairKey `json:"key"`
	BuildWallMS      int64   `json:"build_wall_ms"`
	TestWallMS       int64   `json:"test_wall_ms"`
	ConformanceWallMS int64  `json:"conformance_wall_ms"`
	PeakRSSKiB       int64   `json:"peak_rss_kib"`
}

type PerformancePair struct {
	Before PerformanceSnapshot `json:"before"`
	After  PerformanceSnapshot `json:"after"`
}

type EvidenceTiming struct {
	LookupMS      int64 `json:"lookup_ms"`
	VerificationMS int64 `json:"verification_ms"`
}

type Expectations struct {
	State                State          `json:"state"`
	TestCounts           ExecutionCounts `json:"test_counts"`
	InvalidatedEdgeCount int            `json:"invalidated_edge_count"`
	EconomyState         State          `json:"economy_state"`
}

type Fixture struct {
	Schema          string                `json:"schema"`
	CaseID          string                `json:"case_id"`
	Description     string                `json:"description"`
	InputBindings   InputBindings         `json:"input_bindings"`
	Graph           SemanticChangeGraph  `json:"semantic_change_graph"`
	Tests           []TestSpec            `json:"tests"`
	PriorReceipts   []PriorTestReceipt    `json:"prior_receipts"`
	Counterexamples []Counterexample      `json:"counterexamples"`
	EvidenceTiming  EvidenceTiming        `json:"evidence_timing"`
	PerformancePair *PerformancePair      `json:"performance_pair,omitempty"`
	Expected        Expectations          `json:"expected"`
}

type Corpus struct {
	Schema            string       `json:"schema"`
	CorpusID          string       `json:"corpus_id"`
	DenominatorID     string       `json:"denominator_id"`
	FixedDenominator  int          `json:"fixed_denominator"`
	Cases             []CorpusCase `json:"cases"`
}

type CorpusCase struct {
	Ordinal int    `json:"ordinal"`
	CaseID  string `json:"case_id"`
	Path    string `json:"path"`
	State   State  `json:"state"`
}

type UnknownDetail struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type ActivityDecision struct {
	Ordinal     int            `json:"ordinal"`
	ID          string         `json:"id"`
	Activity    string         `json:"activity"`
	Stage       string         `json:"stage"`
	Step        string         `json:"step"`
	ProofChoice string         `json:"proof_choice"`
	IndicatorClass string      `json:"indicator_class"`
	State       State          `json:"state"`
	Reason      string         `json:"reason"`
	Unknown     *UnknownDetail `json:"unknown,omitempty"`
}

type TestDecision struct {
	TestID                      string   `json:"test_id"`
	OperationID                 string   `json:"operation_id"`
	Status                      string   `json:"status"`
	Affected                    bool     `json:"affected"`
	Reason                      string   `json:"reason"`
	MinimalInvalidationFrontier []string `json:"minimal_invalidation_frontier"`
	PriorReceiptID              string   `json:"prior_receipt_id,omitempty"`
	WallMS                      *int64   `json:"wall_ms,omitempty"`
	PeakRSSKiB                  *int64   `json:"peak_rss_kib,omitempty"`
	ResultDigest                string   `json:"result_digest,omitempty"`
}

type InvalidationFrontierEdge struct {
	EdgeID string `json:"edge_id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind"`
}

type EvidenceResult struct {
	TestID     string `json:"test_id"`
	ReceiptID  string `json:"receipt_id"`
	Valid      bool   `json:"valid"`
	Reason     string `json:"reason"`
}

type MetricComparison struct {
	Metric string `json:"metric"`
	Before int64  `json:"before"`
	After  int64  `json:"after"`
	Delta  int64  `json:"delta"`
	Improved bool  `json:"improved"`
}

type EconomyAssessment struct {
	State        State               `json:"state"`
	Reason       string              `json:"reason"`
	Unknown      *UnknownDetail      `json:"unknown,omitempty"`
	Before       *PerformanceSnapshot `json:"before,omitempty"`
	After        *PerformanceSnapshot `json:"after,omitempty"`
	Comparisons  []MetricComparison  `json:"comparisons"`
}

type ExecutionCounts struct {
	Total        int `json:"total"`
	Executed     int `json:"executed"`
	Reused       int `json:"reused"`
	Skipped      int `json:"skipped"`
	NotObserved  int `json:"not_observed"`
}

type ActivitySummary struct {
	Total   int `json:"total"`
	Closed  int `json:"closed"`
	Unknown int `json:"unknown"`
	Refuted int `json:"refuted"`
}

type Authority struct {
	RepositoryWrites          int `json:"repository_writes"`
	LocalTestExecutions       int `json:"local_test_executions"`
	CrossProjectRequiredGates int `json:"cross_project_required_gates"`
}

type ArtifactBinding struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type Plan struct {
	Schema                string                    `json:"schema"`
	CaseID                string                    `json:"case_id"`
	State                 State                     `json:"state"`
	DecisionReason        string                    `json:"decision_reason"`
	Activities            []ActivityDecision        `json:"activities"`
	Tests                 []TestDecision            `json:"tests"`
	InvalidationFrontier  []InvalidationFrontierEdge `json:"invalidation_frontier"`
	InvalidatedEdgeCount  int                       `json:"invalidated_edge_count"`
	ExecutionCounts       ExecutionCounts           `json:"execution_counts"`
	Evidence              []EvidenceResult          `json:"evidence"`
	EvidenceTiming        EvidenceTiming            `json:"evidence_timing"`
	Economy               EconomyAssessment         `json:"economy"`
	ActivitySummary       ActivitySummary           `json:"activity_summary"`
	OutputArtifacts       int                       `json:"output_artifacts"`
	Dossier               string                    `json:"human_dossier"`
}

type Receipt struct {
	Schema                string          `json:"schema"`
	CaseID                string          `json:"case_id"`
	State                 State           `json:"state"`
	DecisionReason        string          `json:"decision_reason"`
	Source                ArtifactBinding `json:"source"`
	SemanticIR            ArtifactBinding `json:"semantic_ir"`
	GeneratedGo           ArtifactBinding `json:"generated_go"`
	Evaluator             ArtifactBinding `json:"evaluator"`
	Contract              ArtifactBinding `json:"contract"`
	Activities            ActivitySummary `json:"activity_summary"`
	ExecutionCounts       ExecutionCounts `json:"execution_counts"`
	InvalidatedEdgeCount  int             `json:"invalidated_edge_count"`
	EvidenceTiming        EvidenceTiming `json:"evidence_timing"`
	EconomyState          State           `json:"economy_state"`
	OutputArtifacts       int             `json:"output_artifacts"`
	Authority             Authority       `json:"product_authority"`
}

type Evaluation struct {
	Plan    Plan    `json:"plan"`
	Receipt Receipt `json:"receipt"`
}
