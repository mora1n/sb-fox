package merge

import (
	"os"
	"path/filepath"
	"testing"
)

// TestByteExactOrdering verifies Go output is byte-identical to the JS golden,
// which proves object key ordering (risk #1) is preserved, not just structure.
func TestByteExactOrdering(t *testing.T) {
	for _, s := range scenarios {
		s := s
		t.Run(s.name, func(t *testing.T) {
			out := runScenario(t, s)
			compact := mustMarshal(t, out)
			pretty, err := Indent(compact)
			if err != nil {
				t.Fatalf("indent: %v", err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "golden", s.name+".json"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			wantTrim := want
			if len(wantTrim) > 0 && wantTrim[len(wantTrim)-1] == '\n' {
				wantTrim = wantTrim[:len(wantTrim)-1]
			}
			if string(pretty) != string(wantTrim) {
				off := firstDiff(pretty, wantTrim)
				lo := off - 60
				if lo < 0 {
					lo = 0
				}
				t.Errorf("%s: byte mismatch at offset %d\n GO : ...%q...\n JS : ...%q...", s.name, off,
					sliceAround(pretty, lo, off+60), sliceAround(wantTrim, lo, off+60))
			}
		})
	}
}

func firstDiff(a, b []byte) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	off := 0
	for off < limit && a[off] == b[off] {
		off++
	}
	return off
}

func sliceAround(b []byte, lo, hi int) string {
	if lo < 0 {
		lo = 0
	}
	if hi > len(b) {
		hi = len(b)
	}
	return string(b[lo:hi])
}
