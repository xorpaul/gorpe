//go:build !windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestExecuteCommandLingeringChild(t *testing.T) {
	// The command exits 0 but leaves a child holding stderr open; the result
	// must be the command's own, returned after WaitDelay, not UNKNOWN.
	start := time.Now()
	r := ExecuteCommand([]string{"sh", "-c", "sleep 5 >&2 & echo OK - fine"}, 60, true, false)
	if d := time.Since(start); d > 4*time.Second {
		t.Fatalf("took %v, want about commandKillGrace", d)
	}
	if r.ReturnCode != 0 || r.Output != "OK - fine\n" {
		t.Fatalf("rc=%d output=%q", r.ReturnCode, r.Output)
	}

	r = ExecuteCommand([]string{"sh", "-c", "sleep 5 >&2 & echo CRITICAL; exit 2"}, 60, true, false)
	if r.ReturnCode != 2 || !strings.HasPrefix(r.Output, "CRITICAL") {
		t.Fatalf("rc=%d output=%q", r.ReturnCode, r.Output)
	}
}

func TestExecuteCommandTimeout(t *testing.T) {
	start := time.Now()
	r := ExecuteCommand([]string{"sh", "-c", "sleep 30"}, 1, true, false)
	if d := time.Since(start); d > 4*time.Second {
		t.Fatalf("took %v", d)
	}
	if r.ReturnCode != 3 || !strings.Contains(r.Output, "timed out") {
		t.Fatalf("rc=%d output=%q", r.ReturnCode, r.Output)
	}
}
