package main

import (
	"cmp"
	"fmt"
	"log"
	"maps"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	h "github.com/xorpaul/gohelper"
)

func httpHandler(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	method := r.Method
	rid := h.RandSeq()
	checkHostnames, err := net.LookupAddr(ip)
	checkHostname := ip
	if err != nil {
		log.Println(rid + " Error while resolving requesting ip: " + ip + " Error: " + err.Error())
	} else if len(checkHostnames) > 0 {
		checkHostname = checkHostnames[0]
	}
	h.Debugf(rid + " Incoming " + method + " request from IP: " + ip + " (" + checkHostname + ")")

	switch method {
	case "GET", "POST":
		if !slices.Contains(config.Main.AllowedIPs, ip) {
			forbiddenRequestCounter++
			log.Print(rid + " Incoming IP " + ip + " (" + checkHostname + ") not in allowed_ips config setting!")
			checkResult{"Your IP " + ip + " (" + checkHostname + ") is not allowed to query anything from me!", 3}.Exit(w, r)
			return
		}

		if config.Main.VerifyClientCert == 1 && len(config.Main.ClientAuthDNs) > 0 {
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				forbiddenRequestCounter++
				log.Print(rid + " No client certificate presented")
				checkResult{"No client certificate presented", 3}.Exit(w, r)
				return
			}
			if err := checkCertAuth(r.TLS.PeerCertificates[0]); err != nil {
				forbiddenRequestCounter++
				log.Printf("%s Client cert auth failed for %s: %v", rid, ip, err)
				checkResult{"Client certificate not authorized: " + err.Error(), 3}.Exit(w, r)
				return
			}
		}

		h.Debugf(rid + " Request path: " + r.URL.Path)

		r.ParseForm()
		command := r.URL.Path[1:]
		forbidden := nastyMetachars
		if slices.Contains(config.Main.RelaxedArgCommands, command) {
			forbidden = relaxedMetachars
		}
		var cmdArguments []string
		for _, k := range sortedArgKeys(r.Form) {
			value := strings.Join(r.Form[k], "")
			h.Debugf(rid + " Found command argument " + k + ": " + value)
			if i := strings.IndexAny(value, forbidden); i >= 0 {
				forbiddenRequestCounter++
				msg := fmt.Sprintf("Found nasty meta character %q at position %d in command argument %s! Forbidden characters for %s: %q", value[i], i, k, command, forbidden)
				log.Print(rid + " " + msg)
				checkResult{msg, 3}.Exit(w, r)
				return
			}
			// Empty arguments are skipped, as before: the next one fills the placeholder.
			if value != "" {
				cmdArguments = append(cmdArguments, value)
			}
		}

		if r.URL.Path == "/" {
			requestCounter++
			gorpe_uptime := strconv.FormatFloat(time.Since(start).Seconds(), 'f', 1, 64) + "s"
			perfData := "|gorpe_uptime=" + gorpe_uptime
			perfData += " requests=" + strconv.Itoa(requestCounter)
			perfData += " forbiddenrequests=" + strconv.Itoa(forbiddenRequestCounter)
			perfData += " failedrequests=" + strconv.Itoa(failedRequestCounter)
			sslText := "SSL client certificate verification disabled"
			if config.Main.VerifyClientCert == 1 {
				sslText = "SSL client certificate verification enabled"
			}
			checkResult{"GORPE version " + buildversion + " with TCP keepalive period " + keepAlivePeriod.String() + " HTTP/2 " + sslText + " Build time: " + buildtime + "GORPE uptime: " + gorpe_uptime + perfData, 0}.Exit(w, r)
			return
		}

		if _, ok := config.Commands[command]; ok {
			h.Debugf(rid + " Found " + strconv.Itoa(len(cmdArguments)) + " command arguments in this command")
			h.Debugf(rid + " Got command from config: " + config.Commands[command])
			argv, err := buildArgv(config.Commands[command], cmdArguments)
			if err != nil {
				failedRequestCounter++
				log.Print(rid + " " + err.Error())
				checkResult{"UNKNOWN: " + err.Error(), 3}.Exit(w, r)
			} else {
				h.Debugf(rid + " Replacing arguments and executing: " + shellquote.Join(argv...))
				before := time.Now()
				cr := ExecuteCommand(argv, config.Main.CommandTimeout, true, wantsJSON(r))
				//strconv.FormatFloat(time.Since(before).Seconds(), 'f', 1, 64)
				executionTime := time.Since(before).Seconds()
				if len(cr.Output) == 0 {
					cr.Output += "Received no text"
				}
				// Making sure that the check script output ends with a newline char
				if cr.Output[len(cr.Output)-1] != 10 {
					cr.Output += "\n"
				}
				h.Debugf(rid + " Received check command: " + command + " from " + ip + " (" + checkHostname + ") got return code: " + strconv.Itoa(cr.ReturnCode) + " and took " + strconv.FormatFloat(executionTime, 'f', 1, 64) + "s")
				h.Debugf(rid + " Received check command: " + command + " from " + ip + " (" + checkHostname + ") got output: " + cr.Output)
				checkResult{cr.Output, cr.ReturnCode}.ExitWithStreams(w, r, cr)
			}
		} else {
			failedRequestCounter++
			log.Print(rid + " Command " + command + " not found!")
			checkResult{"UKNOWN: Command " + command + " not found!", 3}.Exit(w, r)
			return
		}
	default:
		forbiddenRequestCounter++
		log.Print(rid + " Incoming HTTP method " + method + " from IP " + ip + " not supported!")
		checkResult{"HTTP method " + method + " not supported!", 3}.Exit(w, r)
		return
	}

}

// sortedArgKeys returns the form keys ordered by their numeric suffix
// (arg1, arg2, ..., arg10), so $ARG$ placeholders are filled in the order the
// client sent them rather than in Go's random map iteration order.
func sortedArgKeys(form url.Values) []string {
	keys := slices.Collect(maps.Keys(form))
	argNum := func(k string) (int, bool) {
		n, err := strconv.Atoi(strings.TrimPrefix(k, "arg"))
		return n, err == nil
	}
	slices.SortFunc(keys, func(a, b string) int {
		na, oka := argNum(a)
		nb, okb := argNum(b)
		if oka && okb && na != nb {
			return cmp.Compare(na, nb)
		}
		return strings.Compare(a, b)
	})
	return keys
}
