//go:build windows
// +build windows

package main

import "log"

func setupSyslog() error {
	log.Print("logging to STDOUT (syslog not available on Windows)")
	return nil
}
