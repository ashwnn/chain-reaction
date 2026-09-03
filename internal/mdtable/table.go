package mdtable

import (
	"fmt"
	"io"
	"strings"
)

func Write(w io.Writer, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	fmt.Fprintf(w, "| %s |\n", strings.Join(escapeAll(headers), " | "))
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	fmt.Fprintf(w, "| %s |\n", strings.Join(seps, " | "))
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			if i < len(row) {
				cells[i] = escape(row[i])
			}
		}
		fmt.Fprintf(w, "| %s |\n", strings.Join(cells, " | "))
	}
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func escapeAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = escape(s)
	}
	return out
}
