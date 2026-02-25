package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Record struct {
	Timestamp time.Time      `json:"timestamp"`
	Step      string         `json:"step"`
	Data      map[string]any `json:"data"`
}

type Collector struct {
	mu       sync.Mutex
	dir      string
	jsonl    *os.File
	jsonlEnc *json.Encoder
}

func NewCollector(dir string) (*Collector, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create evidence directory: %w", err)
	}

	path := filepath.Join(dir, "evidence.jsonl")
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create evidence file: %w", err)
	}

	return &Collector{
		dir:      dir,
		jsonl:    f,
		jsonlEnc: json.NewEncoder(f),
	}, nil
}

func (c *Collector) Record(step string, data map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	rec := Record{
		Timestamp: time.Now().UTC(),
		Step:      step,
		Data:      data,
	}

	if err := c.jsonlEnc.Encode(rec); err != nil {
		return fmt.Errorf("write evidence record: %w", err)
	}

	return nil
}

func (c *Collector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.jsonl == nil {
		return nil
	}
	err := c.jsonl.Close()
	c.jsonl = nil
	return err
}

func (c *Collector) Dir() string {
	return c.dir
}
