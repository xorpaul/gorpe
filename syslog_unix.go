//go:build !windows
// +build !windows

package main

import (
	"log"
	"log/syslog"
)

func setupSyslog() error {
	logwriter, err := syslog.New(syslog.LOG_NOTICE, "gorpe")
	if err != nil {
		return err
	}
	log.SetOutput(logwriter)
	log.Print("logging to syslog")
	return nil
}