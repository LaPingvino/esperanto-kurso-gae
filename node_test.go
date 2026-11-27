package main

import (
	"os/exec"
	"testing"
)

// The browser's spaced repetition (static/fsrs.js, static/ripetoj.js) is
// tested by node, as on angla.org: FSRS against py-fsrs's own values, the
// cards' order and the rebuild from the server's trace. Skipped without node.
func TestRipetojNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node")
	}
	for _, f := range []string{"tools/test-fsrs.mjs", "tools/test-ripetoj.mjs"} {
		out, err := exec.Command(node, f).CombinedOutput()
		if err != nil {
			t.Errorf("%s: %v\n%s", f, err, out)
		}
	}
}
