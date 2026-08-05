package api_test

// HTTP-edge tests for GET /v1/puzzles. The handler serves the embedded seed catalog grouped by
// tier as {sections:[{name,puzzles}]} — the exact shape the frontend dropdown consumes.
//
//	func PuzzlesHandler() http.Handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottbushyhead/sudoku-flow/internal/api"
)

func getPuzzles(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/puzzles", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// The catalog returns 200 with the four tiers in file order, their expected display names, the
// expected per-tier counts (Original 25; Medium/Hard/Very Hard 10 each), and every puzzle a
// raw 81-char string.
func TestPuzzles_CatalogShapeAndCounts(t *testing.T) {
	rr := getPuzzles(t, api.PuzzlesHandler())
	if rr.Code != http.StatusOK {
		t.Fatalf("got HTTP %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json...", ct)
	}

	var cat api.PuzzleCatalog
	if err := json.Unmarshal(rr.Body.Bytes(), &cat); err != nil {
		t.Fatalf("body not a PuzzleCatalog: %v", err)
	}

	want := []struct {
		name  string
		count int
	}{
		{"Original", 25},
		{"Medium", 10},
		{"Hard", 10},
		{"Very Hard", 10},
	}
	if len(cat.Sections) != len(want) {
		t.Fatalf("got %d sections, want %d (%v)", len(cat.Sections), len(want), cat.Sections)
	}
	for i, w := range want {
		got := cat.Sections[i]
		if got.Name != w.name {
			t.Errorf("section %d: name = %q, want %q", i, got.Name, w.name)
		}
		if len(got.Puzzles) != w.count {
			t.Errorf("section %q: %d puzzles, want %d", w.name, len(got.Puzzles), w.count)
		}
		for j, p := range got.Puzzles {
			if len(p) != 81 {
				t.Errorf("section %q puzzle %d: len = %d, want 81 (%q)", w.name, j, len(p), p)
			}
		}
	}
}

// The exact wire shape the frontend is coded to: a top-level "sections" array whose items each
// carry "name" and "puzzles" keys.
func TestPuzzles_WireContractKeys(t *testing.T) {
	rr := getPuzzles(t, api.PuzzlesHandler())
	var raw struct {
		Sections []map[string]json.RawMessage `json:"sections"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body not a {sections:[...]} object: %v", err)
	}
	if len(raw.Sections) == 0 {
		t.Fatal("sections array is empty")
	}
	for i, s := range raw.Sections {
		if _, ok := s["name"]; !ok {
			t.Errorf("section %d missing \"name\" key", i)
		}
		if _, ok := s["puzzles"]; !ok {
			t.Errorf("section %d missing \"puzzles\" key", i)
		}
	}
}
