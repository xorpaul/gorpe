package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
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
	var body bytes.Buffer
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
		json.NewEncoder(&body).Encode(res)
	} else {
		if !h.InBetween(cr.returncode, 0, 3) {
			cr.returncode = 3
		}
		w.Header().Set("Content-Type", "text/plain")
		body.WriteString(cr.text + "\nResult Code: " + strconv.Itoa(cr.returncode) + "\n")
	}
	writeBody(w, r, body.Bytes())
}

// defaultCompressMinBytes is used when compress_min_bytes is not set.
const defaultCompressMinBytes = 1024

// compressMinBytes returns the body size from which responses are gzipped;
// 0 disables compression.
func compressMinBytes() int {
	if config.Main.CompressMinBytes == nil {
		return defaultCompressMinBytes
	}
	return *config.Main.CompressMinBytes
}

// acceptsGzip reports whether the client listed gzip in Accept-Encoding
// without q=0.
func acceptsGzip(r *http.Request) bool {
	for _, v := range r.Header.Values("Accept-Encoding") {
		for _, part := range strings.Split(v, ",") {
			fields := strings.Split(part, ";")
			if !strings.EqualFold(strings.TrimSpace(fields[0]), "gzip") {
				continue
			}
			rejected := false
			for _, p := range fields[1:] {
				p = strings.ReplaceAll(strings.TrimSpace(p), " ", "")
				if strings.HasPrefix(p, "q=0") && strings.Trim(strings.TrimPrefix(p, "q="), "0.") == "" {
					rejected = true
				}
			}
			if !rejected {
				return true
			}
		}
	}
	return false
}

// writeBody writes a fully buffered response body, gzip-compressed when the
// client accepts it and the body reaches compress_min_bytes.
func writeBody(w http.ResponseWriter, r *http.Request, body []byte) {
	min := compressMinBytes()
	w.Header().Add("Vary", "Accept-Encoding")
	if min <= 0 || len(body) < min || !acceptsGzip(r) {
		w.Write(body)
		return
	}
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	gz.Write(body)
	if err := gz.Close(); err != nil {
		w.Write(body)
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")
	w.Write(buf.Bytes())
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
	} else if errors.Is(err, exec.ErrWaitDelay) && c.ProcessState != nil {
		// The command itself exited, but a child it left running (e.g. a
		// background job inheriting stdout/stderr) kept the output pipe open
		// past WaitDelay. That is the command's own business: report its exit
		// code and the output written so far instead of failing the check.
		er.ReturnCode = c.ProcessState.ExitCode()
		log.Print("WARN: " + command + " exited with " + strconv.Itoa(er.ReturnCode) + " but left a child process holding its output open; returning without waiting for it")
	} else if err != nil {
		er.ReturnCode = 3
		er.Output = "UNKNOWN: could not execute command: " + err.Error() + "\n" + er.Output
	}
	h.Verbosef("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	return er
}
