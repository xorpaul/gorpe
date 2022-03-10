package main

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	h "github.com/xorpaul/gohelper"
)

func httpHandler(w http.ResponseWriter, r *http.Request) {
	ip := strings.Split(r.RemoteAddr, ":")[0]
	method := r.Method
	rid := h.RandSeq()
	checkHostnames, err := net.LookupAddr(ip)
	if err != nil {
		log.Println(rid + " Error while resolving requesting ip: " + ip + " Error: " + err.Error())
	}
	checkHostname := checkHostnames[0]
	h.Debugf(rid + " Incoming " + method + " request from IP: " + ip + " (" + checkHostname + ")")

	allowed := false
	switch method {
	case "GET", "POST":
		for _, allowedHost := range config.Main.AllowedHosts {
			if ip == allowedHost {
				allowed = true
			}
		}
		if !allowed {
			forbiddenRequestCounter++
			log.Print(rid + " Incoming IP " + ip + " (" + checkHostname + ") not in allowed_hosts config setting!")
			checkResult{"Your IP " + ip + " (" + checkHostname + ") is not allowed to query anything from me!", 3}.Exit(w)
			return
		}

		h.Debugf(rid + " Request path: " + r.URL.Path)

		r.ParseForm()
		command := r.URL.Path[1:]
		var cmdArguments []string
		for k, v := range r.Form {
			value := strings.Join(v, "")
			h.Debugf(rid + " Found command argument " + k + ": " + value)
			if strings.ContainsAny(value, nastyMetachars) {
				forbiddenRequestCounter++
				log.Print(rid + " Command arguments are not allowed to contain any of: " + nastyMetachars)
				checkResult{"Found nasty meta character in command arguments!", 3}.Exit(w)
				return
			}
			cmdArguments = append(cmdArguments, value)
		}

		if r.URL.Path == "/" {
			requestCounter++
			perfData := "|gorpe_uptime=" + strconv.FormatFloat(time.Since(start).Seconds(), 'f', 1, 64) + "s"
			perfData += " requests=" + strconv.Itoa(requestCounter)
			perfData += " forbiddenrequests=" + strconv.Itoa(forbiddenRequestCounter)
			perfData += " failedrequests=" + strconv.Itoa(failedRequestCounter)
			sslText := "SSL client certificate verification disabled"
			if config.Main.VerifyClientCert == 1 {
				sslText = "SSL client certificate verification enabled"
			}
			checkResult{"GORPE version " + version + " HTTP/2 " + sslText + " Build time: " + buildtime + perfData, 0}.Exit(w)
			return
		}

		if _, ok := config.Commands[command]; ok {
			argCount := strings.Count(config.Commands[command], "$ARG$")
			h.Debugf(rid + " Found " + strconv.Itoa(len(cmdArguments)) + " command arguments in this command")
			if argCount > len(cmdArguments) {
				failedRequestCounter++
				log.Print(rid + " Not enough command arguments! Expected " + strconv.Itoa(argCount) + " and found " + strconv.Itoa(len(cmdArguments)))
				checkResult{"UNKNOWN: Not enough command arguments! Expected " + strconv.Itoa(argCount) + " and found " + strconv.Itoa(len(cmdArguments)), 3}.Exit(w)
			} else {
				cmdString := config.Commands[command]
				h.Debugf(rid + " Got command from config: " + cmdString)
				for _, arg := range cmdArguments {
					if arg != "" {
						cmdString = strings.Replace(cmdString, "$ARG$", arg, 1)
						h.Debugf(rid + " Replacing $ARG$ with " + arg + " resulting in " + cmdString)
					}
				}
				h.Debugf(rid + " Replacing arguments and executing: " + cmdString)
				before := time.Now()
				cr := ExecuteCommand(cmdString, config.Main.CommandTimeout, true)
				//strconv.FormatFloat(time.Since(before).Seconds(), 'f', 1, 64)
				executionTime := time.Since(before).Seconds()
				if len(cr.Output) == 0 {
					cr.Output += "Received no text"
				}
				// Making sure that the check script output ends with a newline char
				if cr.Output[len(cr.Output)-1] != 10 {
					cr.Output += "\n"
				}
				log.Print(rid + " Received check command: " + command + " from " + ip + " (" + checkHostname + ") got return code: " + strconv.Itoa(cr.ReturnCode) + " and took " + strconv.FormatFloat(executionTime, 'f', 1, 64) + "s")
				log.Print(rid + " Received check command: " + command + " from " + ip + " (" + checkHostname + ") got output: " + cr.Output)
				checkResult{cr.Output, cr.ReturnCode}.Exit(w)
			}
		} else {
			failedRequestCounter++
			log.Print(rid + " Command " + command + " not found!")
			checkResult{"UKNOWN: Command " + command + " not found!", 3}.Exit(w)
			return
		}
	default:
		forbiddenRequestCounter++
		log.Print(rid + " Incoming HTTP method " + method + " from IP " + ip + " not supported!")
		checkResult{"HTTP method " + method + " not supported!", 3}.Exit(w)
		return
	}

}
