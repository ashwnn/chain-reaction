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
	mu     sync.Mutex
	dir    string
	path   string
	closed bool
}

func NewCollector(dir string) (*Collector, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create evidence directory: %w", err)
	}

	path := filepath.Join(dir, "evidence.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create evidence file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("close evidence file: %w", err)
	}

	return &Collector{dir: dir, path: path}, nil
}

func (c *Collector) Record(step string, data map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("collector closed")
	}

	f, err := os.OpenFile(c.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open evidence file: %w", err)
	}
	defer f.Close()

	rec := Record{
		Timestamp: time.Now().UTC(),
		Step:      step,
		Data:      data,
	}
	if err := json.NewEncoder(f).Encode(rec); err != nil {
		return fmt.Errorf("write evidence record: %w", err)
	}
	return nil
}

func (c *Collector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *Collector) Dir() string {
	return c.dir
}
