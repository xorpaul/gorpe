package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kballard/go-shellquote"
	h "github.com/xorpaul/gohelper"
)

// ExecResult contains the exit code and output of an external command (e.g. git)
type ExecResult struct {
	ReturnCode int
	Output     string
}

// Exit is a helper function to output a check result in a standardized way
func (cr checkResult) Exit(w http.ResponseWriter) {
	if !(cr.returncode != 0 || cr.returncode != 1 || cr.returncode != 2 || cr.returncode != 3) {
		cr.returncode = 3
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(cr.text + "\nResult Code: " + strconv.Itoa(cr.returncode) + "\n"))
}

func ExecuteCommand(command string, timeout int, allowFail bool) ExecResult {
	h.Debugf("Executing " + command)
	parts := strings.SplitN(command, " ", 2)
	cmd := parts[0]
	cmdArgs := []string{}
	if len(parts) > 1 {
		args, err := shellquote.Split(parts[1])
		if err != nil {
			h.Debugf("err: " + fmt.Sprint(err))
		} else {
			cmdArgs = args
		}
	}

	before := time.Now()
	out, err := exec.Command(cmd, cmdArgs...).CombinedOutput()
	duration := time.Since(before).Seconds()
	er := ExecResult{0, string(out)}
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		er.ReturnCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
		h.Debugf("Setting return code to " + strconv.Itoa(er.ReturnCode))
	}
	h.Verbosef("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	return er
}
