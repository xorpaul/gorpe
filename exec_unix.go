//go:build !windows

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// setupCommandCancel runs c in its own process group and, on timeout, sends
// SIGTERM to the whole group so pipeline children holding the output pipe die
// too. sudo relays SIGTERM to its (root) child; it cannot relay SIGKILL.
// Whatever ignores SIGTERM gets a group-wide SIGKILL after commandKillGrace.
func setupCommandCancel(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		pgid := -c.Process.Pid
		time.AfterFunc(commandKillGrace, func() { syscall.Kill(pgid, syscall.SIGKILL) })
		return syscall.Kill(pgid, syscall.SIGTERM)
	}
}
