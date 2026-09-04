package benchmark

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateEvaluationInventorySeparates48PrivateCommitments(t *testing.T) {
	root := t.TempDir()
	manifest, err := CreateEvaluationInventory(root, bytes.NewReader(bytes.Repeat([]byte{7}, 32*len(AllArchetypes())*privateSeedsPerArchetype)))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(manifest.Commitments), len(AllArchetypes())*(2+2*privateSeedsPerArchetype); got != want {
		t.Fatalf("commitments = %d, want %d", got, want)
	}
	seeds, err := filepath.Glob(filepath.Join(root, "*.seed"))
	if err != nil || len(seeds) != len(AllArchetypes())*privateSeedsPerArchetype {
		t.Fatalf("private seeds = %d, err = %v", len(seeds), err)
	}
	body, err := CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, seedPath := range seeds {
		seed, err := os.ReadFile(seedPath)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(body, seed) {
			t.Fatalf("commitment leaks %s", seedPath)
		}
	}
}
