package evidence

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Record struct {
	Timestamp time.Time      `json:"timestamp"`
	Step      string         `json:"step"`
	Data      map[string]any `json:"data"`
	Sequence  int            `json:"sequence"`
	PrevHash  string         `json:"prev_hash,omitempty"`
	Hash      string         `json:"hash"`
}

type Collector struct {
	mu       sync.Mutex
	dir      string
	path     string
	sequence int
	lastHash string
	closed   bool
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

	collector := &Collector{dir: dir, path: path}
	if info, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("stat evidence file: %w", err)
	} else if info.Size() > 0 {
		last, err := VerifyEvidenceLog(path)
		if err != nil {
			return nil, err
		}
		collector.sequence = last.Sequence
		collector.lastHash = last.Hash
	}
	return collector, nil
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
		Sequence:  c.sequence + 1,
		PrevHash:  c.lastHash,
	}
	hash, err := recordHash(rec)
	if err != nil {
		return err
	}
	rec.Hash = hash
	if err := json.NewEncoder(f).Encode(rec); err != nil {
		return fmt.Errorf("write evidence record: %w", err)
	}
	c.sequence = rec.Sequence
	c.lastHash = rec.Hash
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

// VerifyEvidenceLog rejects tampering, truncation, reordering, duplicate
// sequence numbers, and malformed records. It returns the terminal record.
func VerifyEvidenceLog(path string) (Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return Record{}, fmt.Errorf("open evidence file: %w", err)
	}
	defer f.Close()
	decoder := json.NewDecoder(bufio.NewReader(f))
	var previous Record
	count := 0
	for {
		var record Record
		err := decoder.Decode(&record)
		if err == io.EOF {
			break
		}
		if err != nil {
			return Record{}, fmt.Errorf("decode evidence record %d: %w", count+1, err)
		}
		if record.Sequence != count+1 || record.Step == "" || record.Hash == "" || (count > 0 && record.PrevHash != previous.Hash) || (count == 0 && record.PrevHash != "") {
			return Record{}, fmt.Errorf("invalid evidence chain at sequence %d", count+1)
		}
		expected, err := recordHash(record)
		if err != nil {
			return Record{}, err
		}
		if record.Hash != expected {
			return Record{}, fmt.Errorf("evidence hash mismatch at sequence %d", record.Sequence)
		}
		previous = record
		count++
	}
	if count == 0 {
		return Record{}, fmt.Errorf("evidence log is empty")
	}
	return previous, nil
}

func recordHash(record Record) (string, error) {
	record.Hash = ""
	body, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("marshal evidence record for hash: %w", err)
	}
	digest := sha256.Sum256(body)
	return fmt.Sprintf("%x", digest[:]), nil
}
