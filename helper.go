package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/kballard/go-shellquote"
	h "github.com/xorpaul/gohelper"
)

// ExecResult contains the exit code and output of an external command (e.g. git)
type ExecResult struct {
	ReturnCode int
	Output     string
	// Stdout and Stderr are only filled when ExecuteCommand ran with
	// splitStreams; Output always holds the combined, interleaved stream.
	Split  bool
	Stdout string
	Stderr string
}

func wantsJSON(r *http.Request) bool {
	return r != nil && r.Header.Get("Accept") == "application/json"
}

// lockedBuffer is a bytes.Buffer safe for the concurrent writes exec makes
// when Stdout and Stderr are different writers.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// Exit writes a check result. When the caller sends Accept: application/json
// it returns {"exit_code": N, "output": "..."} with the raw exit code (not
// clamped), so callers like gorpe-mcp get puppet exit codes 4 and 6 intact.
// The legacy text protocol keeps the 0-3 clamp for Nagios/Icinga compat.
func (cr checkResult) Exit(w http.ResponseWriter, r *http.Request) {
	cr.exit(w, r, nil)
}

// ExitWithStreams is Exit for executed commands: JSON clients additionally get
// "stdout" and "stderr" fields next to the combined "output".
func (cr checkResult) ExitWithStreams(w http.ResponseWriter, r *http.Request, er ExecResult) {
	cr.exit(w, r, &er)
}

func (cr checkResult) exit(w http.ResponseWriter, r *http.Request, er *ExecResult) {
	if wantsJSON(r) {
		res := map[string]interface{}{
			"exit_code": cr.returncode,
			"output":    cr.text,
		}
		if er != nil && er.Split {
			res["stdout"] = er.Stdout
			res["stderr"] = er.Stderr
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
		return
	}
	if !h.InBetween(cr.returncode, 0, 3) {
		cr.returncode = 3
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(cr.text + "\nResult Code: " + strconv.Itoa(cr.returncode) + "\n"))
}

// commandKillGrace is how long a timed-out command gets after SIGTERM before
// it is killed and its output pipes are closed. It must stay below the 5s of
// slack the HTTP server's WriteTimeout gives on top of command_timeout.
const commandKillGrace = 2 * time.Second

// ExecuteCommand runs argv (built by buildArgv) without a shell. With splitStreams, stdout and
// stderr are also captured separately (used for JSON clients); otherwise both
// share one pipe as before, which keeps their exact interleaving for the
// legacy text protocol.
func ExecuteCommand(argv []string, timeout int, allowFail bool, splitStreams bool) ExecResult {
	command := shellquote.Join(argv...)
	h.Debugf("Executing " + command)
	if len(argv) == 0 {
		return ExecResult{ReturnCode: 3, Output: "UNKNOWN: empty command"}
	}

	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	setupCommandCancel(c)
	c.WaitDelay = commandKillGrace

	var combined lockedBuffer
	var stdout, stderr bytes.Buffer
	if splitStreams {
		c.Stdout = io.MultiWriter(&stdout, &combined)
		c.Stderr = io.MultiWriter(&stderr, &combined)
	} else {
		c.Stdout = &combined
		c.Stderr = &combined
	}

	before := time.Now()
	err := c.Run()
	duration := time.Since(before).Seconds()
	er := ExecResult{ReturnCode: 0, Output: combined.buf.String()}
	if splitStreams {
		er.Split, er.Stdout, er.Stderr = true, stdout.String(), stderr.String()
	}
	if ctx.Err() == context.DeadlineExceeded {
		er.ReturnCode = 3
		er.Output = "UNKNOWN: command timed out after " + strconv.Itoa(timeout) + "s (command_timeout)\n" + er.Output
	} else if msg, ok := err.(*exec.ExitError); ok { // there is error code
		er.ReturnCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
		h.Debugf("Setting return code to " + strconv.Itoa(er.ReturnCode))
	} else if err != nil {
		er.ReturnCode = 3
		er.Output = "UNKNOWN: could not execute command: " + err.Error() + "\n" + er.Output
	}
	h.Verbosef("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	return er
}
