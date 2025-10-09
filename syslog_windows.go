//go:build windows
// +build windows

package main

import "log"

func setupSyslog() error {
	// On Windows, we skip syslog and just use stdout
	log.Print("logging to STDOUT (syslog not available on Windows)")
	return nil
}