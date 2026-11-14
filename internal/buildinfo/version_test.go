package buildinfo

import (
	"strconv"
	"strings"
	"testing"
)

func TestVersionIsSemverTriple(t *testing.T) {
	v := Version
	if v == "" {
		t.Fatal("Version must not be empty")
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		t.Fatalf("Version %q is not a semver triple (x.y.z)", v)
	}
	for i, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			t.Fatalf("part %d (%q) not numeric: %v", i, part, err)
		}
	}
}
