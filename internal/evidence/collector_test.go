package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCollector(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCollector(dir)
	if err != nil {
		t.Fatalf("NewCollector() error = %v", err)
	}
	defer c.Close()

	if c.Dir() != dir {
		t.Errorf("expected dir %s, got %s", dir, c.Dir())
	}

	path := filepath.Join(dir, "evidence.jsonl")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", path)
	}
}

func TestCollector_Record(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCollector(dir)
	if err != nil {
		t.Fatalf("NewCollector() error = %v", err)
	}

	step := "test-step"
	data := map[string]any{"key": "value"}
	if err := c.Record(step, data); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify content
	path := filepath.Join(dir, "evidence.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open evidence file: %v", err)
	}
	defer f.Close()

	var rec Record
	if err := json.NewDecoder(f).Decode(&rec); err != nil {
		t.Fatalf("failed to decode record: %v", err)
	}

	if rec.Step != step {
		t.Errorf("expected step %s, got %s", step, rec.Step)
	}
	if rec.Data["key"] != "value" {
		t.Errorf("expected data key value 'value', got %v", rec.Data["key"])
	}
}

func TestCollector_Close(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCollector(dir)
	if err != nil {
		t.Fatalf("NewCollector() error = %v", err)
	}

	// First close
	if err := c.Close(); err != nil {
		t.Errorf("first Close() error = %v", err)
	}

	// Second close should be a no-op and return nil
	if err := c.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}

	// Record after close should fail
	err = c.Record("after-close", nil)
	if err == nil {
		t.Error("expected Record() to fail after Close(), but it didn't")
	}
}

func TestCollector_RecordError(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCollector(dir)
	if err != nil {
		t.Fatalf("NewCollector() error = %v", err)
	}

	if err := os.Remove(c.path); err != nil {
		t.Fatalf("remove evidence file: %v", err)
	}
	if err := os.Mkdir(c.path, 0o755); err != nil {
		t.Fatalf("occupy evidence path: %v", err)
	}

	err = c.Record("error-step", nil)
	if err == nil {
		t.Error("expected Record() to error when file is closed")
	}
}
