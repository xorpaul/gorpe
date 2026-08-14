package main

import (
	"encoding/json"
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

// Exit writes a check result. When the caller sends Accept: application/json
// it returns {"exit_code": N, "output": "..."} with the raw exit code (not
// clamped), so callers like gorpe-mcp get puppet exit codes 4 and 6 intact.
// The legacy text protocol keeps the 0-3 clamp for Nagios/Icinga compat.
func (cr checkResult) Exit(w http.ResponseWriter, r *http.Request) {
	if r != nil && r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"exit_code": cr.returncode,
			"output":    cr.text,
		})
		return
	}
	if !h.InBetween(cr.returncode, 0, 3) {
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
