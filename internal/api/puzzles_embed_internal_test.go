package api

// Internal (package api) test: the drift guard needs the unexported embedded bytes. It fences
// the one hazard of the embed strategy — a controlled copy of the corpus lives at
// internal/api/puzzles.txt so //go:embed can reach it, and this asserts that copy can never
// silently diverge from the repo-root source of truth.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestPuzzlesEmbed_MatchesRepoRoot byte-compares the embedded copy against the repo-root
// puzzles.txt. If someone edits one and forgets the other, this fails loudly rather than
// shipping a stale catalog in the binary.
func TestPuzzlesEmbed_MatchesRepoRoot(t *testing.T) {
	path := filepath.Join("..", "..", "puzzles.txt")
	root, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading repo-root puzzles at %s: %v", path, err)
	}
	if !bytes.Equal(root, puzzlesData) {
		t.Fatalf("embedded internal/api/puzzles.txt has drifted from repo-root puzzles.txt "+
			"(embedded %d bytes, root %d bytes): re-copy the root file into internal/api/",
			len(puzzlesData), len(root))
	}
}
