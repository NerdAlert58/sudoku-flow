package api

import (
	_ "embed"
	"net/http"
	"strings"
	"sync"
)

// puzzlesData is the seed corpus embedded into the binary. //go:embed can only reference a
// file within the embedding package's own directory subtree — not the repo-root sibling — so
// a controlled copy lives here at internal/api/puzzles.txt. The Vercel serverless runtime (and
// the shipped binary generally) has no reliable working-directory access to the repo-root
// puzzles.txt, so the catalog MUST come from the binary, never os.ReadFile (ADR-0014 embed
// discipline). Drift between this copy and the repo-root source is fenced by
// TestPuzzlesEmbed_MatchesRepoRoot, which byte-compares the two.
//
//go:embed puzzles.txt
var puzzlesData []byte

// puzzleSectionDisplayName maps the corpus's on-disk tier headers (ADR-0019, "# === NAME ===")
// to the display names the GET /v1/puzzles contract emits. A header with no entry here passes
// through verbatim, so an added tier still surfaces rather than silently vanishing.
var puzzleSectionDisplayName = map[string]string{
	"ORIGINAL (unlabeled)": "Original",
	"MEDIUM":               "Medium",
	"HARD":                 "Hard",
	"VERY HARD":            "Very Hard",
}

// puzzleCatalog parses the embedded corpus exactly once. The bytes are static (compiled in), so
// the parse result is memoised with sync.OnceValue (Go 1.21+) rather than re-walked per request.
var puzzleCatalog = sync.OnceValue(func() PuzzleCatalog {
	return parsePuzzleCatalog(puzzlesData)
})

// PuzzlesHandler returns the GET /v1/puzzles handler: the seed catalog grouped by tier, in file
// order, as the frozen {sections:[{name,puzzles}]} contract. It is a pure read of embedded data —
// no request body, no downstream calls — so it cannot fail on input and always returns 200.
// Method enforcement is the mux's job (the "GET /v1/puzzles" pattern rejects non-GET with 405),
// matching every other route in app.routes() — no bespoke method check is duplicated here.
func PuzzlesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, puzzleCatalog())
	})
}

// parsePuzzleCatalog groups the corpus's 81-char data lines under their tier headers, preserving
// file order for both the sections and the puzzles within each. The corpus is sectioned with
// "# === NAME ===" headers; every other '#'-prefixed line and every blank line is non-data.
// CRLF-safe: each line is right-trimmed of \r so a CRLF-terminated .txt parses without a trim at
// the call site (matches the internal/solver section-parsing style, but this is shipped code).
func parsePuzzleCatalog(raw []byte) PuzzleCatalog {
	var sections []PuzzleSection
	cur := -1
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			name, ok := puzzleSectionHeaderName(line)
			if !ok {
				continue // a plain comment, not a tier boundary
			}
			display := name
			if d, found := puzzleSectionDisplayName[name]; found {
				display = d
			}
			sections = append(sections, PuzzleSection{Name: display})
			cur = len(sections) - 1
			continue
		}
		if cur < 0 {
			continue // data before any header: no tier to attach it to
		}
		sections[cur].Puzzles = append(sections[cur].Puzzles, line)
	}
	return PuzzleCatalog{Sections: sections}
}

// puzzleSectionHeaderName extracts NAME from a "# === NAME ===" header, reporting false for any
// other '#'-prefixed line so plain comments are skipped rather than treated as tier boundaries.
func puzzleSectionHeaderName(line string) (string, bool) {
	s := strings.TrimSpace(strings.TrimPrefix(line, "#"))
	if !strings.HasPrefix(s, "===") || !strings.HasSuffix(s, "===") {
		return "", false
	}
	name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "==="), "==="))
	if name == "" {
		return "", false
	}
	return name, true
}
