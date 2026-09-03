package mdtable

import (
	"strings"
	"testing"
)

func TestWrite(t *testing.T) {
	var b strings.Builder
	Write(&b, []string{"A", "B"}, [][]string{{"1", "2|3"}, {"x", "y"}})
	got := b.String()
	wantLines := []string{
		"| A | B |",
		"| --- | --- |",
		"| 1 | 2\\|3 |",
		"| x | y |",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Errorf("missing %q in:\n%s", line, got)
		}
	}
}
