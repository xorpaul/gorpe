//go:build windows

package main

import "os/exec"

// setupCommandCancel keeps exec.CommandContext's default (Process.Kill) on Windows.
func setupCommandCancel(c *exec.Cmd) {}
