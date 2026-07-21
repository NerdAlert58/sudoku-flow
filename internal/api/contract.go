// Package api owns the HTTP edge: the frozen /v1 JSON contract types, the {error, code}
// error envelope, the request-body cap, and the per-route handlers. It is blinded from
// solver internals — only the contract crosses this boundary (ARCHITECTURE.md §Contracts,
// §Components → internal/api).
package api

// APIVersion is the value emitted in the apiVersion field of every /v1 response. Breaking
// changes mint /v2 and bump this (ADR-0010).
const APIVersion = "1"

// ErrorResponse is the single error envelope every handler returns on failure: a
// human-readable message plus a stable machine code (e.g. "invalid_input"). ADR-0011's
// statuses live on the success path; this envelope is the failure path.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// HealthResponse is the GET /v1/health body: a deployment self-identifies with its status,
// Go version, and API version so a future dashboard can segment by host/version (ADR-0010).
type HealthResponse struct {
	Status     string `json:"status"`
	GoVersion  string `json:"goVersion"`
	APIVersion string `json:"apiVersion"`
}

// --- Solve contract (declared now; wired in P-1) -------------------------------------

// SolveRequest is the POST /v1/solve body: a single 81-character puzzle string.
type SolveRequest struct {
	Puzzle string `json:"puzzle"`
}

// SolveResponse is the POST /v1/solve success body. status is one of solved /
// invalid_input / unsolvable / stalled (ADR-0011); the metric quartet (iterations,
// eventCount, candidateChecks, solveTimeMs) is the benchmark instrument (ADR-0007).
type SolveResponse struct {
	APIVersion      string  `json:"apiVersion"`
	Input           string  `json:"input"`
	Status          string  `json:"status"`
	Solved          bool    `json:"solved"`
	Solution        string  `json:"solution"`
	Iterations      int     `json:"iterations"`
	EventCount      int     `json:"eventCount"`
	CandidateChecks int     `json:"candidateChecks"`
	SolveTimeMs     float64 `json:"solveTimeMs"`
	Events          []Event `json:"events"`
}

// Cell is a row/column coordinate (0-based) used by the replayable event log.
type Cell struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Placement records a digit placed into a cell by a technique.
type Placement struct {
	Cell  Cell `json:"cell"`
	Value int  `json:"value"`
}

// Elimination records a single candidate removed from a cell by a technique.
type Elimination struct {
	Cell      Cell `json:"cell"`
	Candidate int  `json:"candidate"`
}

// Event is one replayable step in the solve log: the technique that fired, the cells that
// witnessed it, and its effect (a placement or a set of eliminations), plus the grid after
// the step. This log is the mechanical proof of the logic-only guarantee (EVAL UC-2).
type Event struct {
	Seq          int           `json:"seq"`
	Technique    string        `json:"technique"`
	WitnessCells []Cell        `json:"witnessCells"`
	Placement    *Placement    `json:"placement,omitempty"`
	Eliminations []Elimination `json:"eliminations,omitempty"`
	GridAfter    string        `json:"gridAfter"`
}

// --- Generate contract (declared now; wired in P-2) ----------------------------------

// GenerateRequest is the POST /v1/generate body: the requested difficulty band.
type GenerateRequest struct {
	Difficulty string `json:"difficulty"`
}

// GeneratedPuzzle is the POST /v1/generate success body: the finished puzzle and its grade.
// The internal uniqueness backtracking counter is never surfaced (ARCHITECTURE.md).
type GeneratedPuzzle struct {
	Puzzle     string `json:"puzzle"`
	Difficulty string `json:"difficulty"`
	Grade      string `json:"grade"`
}

// --- Batch contract (declared now; wired in P-3) -------------------------------------

// BatchRequest is the POST /v1/validate-batch body: a list of puzzle strings. The handler
// bounds the body (MaxBytes) and caps len(Puzzles), rejecting over-cap with 413.
type BatchRequest struct {
	Puzzles []string `json:"puzzles"`
}

// BatchItem is one per-puzzle result in a batch response.
type BatchItem struct {
	Puzzle           string  `json:"puzzle"`
	Solved           bool    `json:"solved"`
	SolveTimeMs      float64 `json:"solveTimeMs"`
	Iterations       int     `json:"iterations"`
	HardestTechnique string  `json:"hardestTechnique"`
}

// BatchResult is the POST /v1/validate-batch success body, results in input order.
type BatchResult struct {
	APIVersion  string      `json:"apiVersion"`
	Results     []BatchItem `json:"results"`
	SolvedCount int         `json:"solvedCount"`
	Total       int         `json:"total"`
}
