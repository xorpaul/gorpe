package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"io"
	"log"
	"log/syslog"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"code.google.com/p/gcfg"
	"github.com/kballard/go-shellquote"
	"github.com/xorpaul/gencerts"
)

var mainCfgSection = make(map[string]string)
var commandsCfgSection = make(map[string]string)
var allowedHosts []string
var allowedPushHosts []string
var uploadDir string

// ConfigSettings contianss the key value pairs from the config file
type ConfigSettings struct {
	main     map[string]string
	commands map[string]string
}

// CheckResult represent the result of an check script
type CheckResult struct {
	text       []byte
	returncode int
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	ip := strings.Split(r.RemoteAddr, ":")[0]
	method := r.Method
	rid := randSeq()
	Debugf(rid, "Incoming "+method+" request from IP: "+ip)

	allowed := false
	switch method {
	case "GET":
		for _, allowedHost := range allowedHosts {
			if ip == allowedHost {
				allowed = true
			}
		}
		if !allowed {
			Debugf(rid, "Incoming IP "+ip+" not in allowed_hosts config setting!")
			abortText := "Your IP " + ip + " is not allowed to query anything from me!\n"
			abortText += "Result Code: 3\n"
			w.Write([]byte(abortText))
			return
		}

		Debugf(rid, "Request path: ", r.URL.Path)

		reqParts := strings.Split(r.URL.Path[1:], "/")
		Debugf(rid, "Request path parts are: %q", reqParts)
		command := reqParts[0]
		Debugf(rid, "Found command: ", command)
		cmdArguments := reqParts[1:len(reqParts)]
		Debugf(rid, "Found command arguments: %q", cmdArguments)

		w.Header().Set("Content-Type", "text/plain")

		if _, ok := commandsCfgSection[command]; ok {
			argCount := strings.Count(commandsCfgSection[command], "$ARG$")
			Debugf(rid, "Found %q command arguments in this command", len(cmdArguments))
			if argCount > len(cmdArguments) {
				w.Write([]byte("Too few command arguments!"))
			} else {
				cmdString := commandsCfgSection[command]
				Debugf(rid, "Got command from config: ", cmdString)
				for _, arg := range cmdArguments {
					// TODO: filter nasty chars
					if arg != "" {
						cmdString = strings.Replace(cmdString, "$ARG$", arg, 1)
						Debugf(rid, "Replacing $ARG$ with %q results in %q", arg, cmdString)
					}
				}
				Debugf(rid, "Replacing arguments and executing: ", cmdString)
				cr := execCommand(cmdString, rid)
				if len(cr.text) == 0 {
					cr.text = append(cr.text, "Received no text"...)
				}
				// Making sure that the check script output ends with a newline char
				if cr.text[len(cr.text)-1] != 10 {
					cr.text = append(cr.text, "\n"...)
				}
				output := append(cr.text, "Returncode: "...)
				output = strconv.AppendInt(output, int64(cr.returncode), 10)
				output = append(output, "\n"...)
				w.Write(output)
			}
		} else {
			text := append([]byte("UKNOWN: Command "), command...)
			text = append(text, " not found!\nReturncode: 0"...)
			w.Write(text)
		}
	case "POST":
		for _, allowedPushHost := range allowedPushHosts {
			Debugf(rid, "comparing ", ip, " == ", allowedPushHost)
			if ip == allowedPushHost {
				allowed = true
			}
		}
		if !allowed {
			Debugf(rid, "Incoming IP "+ip+" not in allowed_push_hosts config setting!")
			abortText := "Your IP " + ip + " is not allowed to upload!\n"
			abortText += "Result Code: 3\n"
			w.Write([]byte(abortText))
			return
		}

		//get the multipart reader for the request.
		reader, err := r.MultipartReader()
		if err != nil {
			Debugf(rid, "Error while reading the Upload request: ", err)
			abortText := "Error while reading the Upload request!\n"
			abortText += "Result Code: 3\n"
			w.Write([]byte(abortText))
			return
		}

		//copy each part to destination.
		fileName := ""
		sha256sum := ""
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			fileName = part.FileName()
			//if part.FileName() is empty, skip this iteration.
			if fileName == "" {
				continue
			}
			dst, err := os.Create(uploadDir + fileName)
			defer dst.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// get sha256 checksum
			hash := sha256.New()
			target := io.TeeReader(part, hash)
			if _, err := io.Copy(dst, target); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sha256sum = hex.EncodeToString(hash.Sum(nil))

			// ensure that the file is executable
			if err := os.Chmod(uploadDir+fileName, 0775); err != nil {
				Debugf(rid, "Error while changing permissions", err)
			}

		}

		// add new file to running config
		checkCommand := r.URL.Path[1:]
		commandsCfgSection[checkCommand] = uploadDir + fileName

		Debugf(rid, "Check Command:", checkCommand, " File ", fileName, "successfully uploaded and saved as ", uploadDir+fileName, " sha256sum: ", sha256sum)
		text := "File " + fileName + " uploaded successfully, sha256sum: " + sha256sum + "\n"
		text += "Result Code: 0\n"
		w.Write([]byte(text))
		return

	default:
		Debugf(rid, "Incoming HTTP method "+method+" from IP "+ip+" not supported!")
		abortText := "HTTP method " + method + " not supported!\n"
		abortText += "Result Code: 3\n"
		w.Write([]byte(abortText))
		return
	}

}

func execCommand(cmdString string, rid string) CheckResult {
	returncode := 3
	parts := strings.SplitN(cmdString, " ", 2)
	checkScript := parts[0]
	checkArguments := []string{}

	if len(parts) > 1 {
		//checkArguments, err := shellquote.Split(strings.Join(parts[1:len(parts)], " "))
		foobar, err := shellquote.Split(parts[1])
		if err != nil {
			Debugf(rid, "err: ", err)
		} else {
			checkArguments = foobar
		}
		Debugf(rid, "checkArguments are %q: ", checkArguments)
	}

	Debugf(rid, "checkScript: ", checkScript)
	Debugf(rid, "checkArguments are %q: ", checkArguments)

	out, err := exec.Command(checkScript, checkArguments...).Output()

	Debugf(rid, "out: ", string(out))
	if err != nil {
		Debugf(rid, "err: ", err)
		if out == nil {
			out = []byte("UKNOWN: unknown output\n")
		}
		Debugf(rid, "out: ", string(out))
	} else {
		if msg, ok := err.(*exec.ExitError); ok { // there is error code
			returncode = msg.Sys().(syscall.WaitStatus).ExitStatus()
		} else {
			returncode = 0
		}
	}

	Debugf(rid+"Got output: ", string(out[:]))
	Debugf(rid+"Got return code: ", returncode)
	return CheckResult{out, returncode}
}

// readConfigfile creates the mainCfgSection and commandsCfgSection structs
// from the gorpe config file
func readConfigfile(configFile string, debugFlag bool) ConfigSettings {
	var cfg = &struct {
		Main struct {
			gcfg.Idxer
			Vals map[gcfg.Idx]*string
		}
		Commands struct {
			gcfg.Idxer
			Vals map[gcfg.Idx]*string
		}
	}{}
	if err := gcfg.ReadFileInto(cfg, configFile); err != nil {
		log.Print("There was an error parsing the configfile: ", err)
	}

	cfgMain := &cfg.Main
	for _, n := range cfgMain.Names() {
		mainCfgSection[n] = *cfgMain.Vals[cfgMain.Idx(n)]
	}

	if debugFlag {
		if mainCfgSection["debug"] == "0" {
			log.Print("overriding debug config file setting, because debug flag was set")
		}
		mainCfgSection["debug"] = "1"
	}

	Debugf("Found main config settings:")
	for _, n := range cfgMain.Names() {
		Debugf(n, " = ", *cfgMain.Vals[cfgMain.Idx(n)])
	}

	if allowed_hosts, ok := mainCfgSection["allowed_hosts"]; ok {
		allowedHosts = strings.Split(allowed_hosts, ",")
		Debugf("allowedHosts:", allowedHosts)
	} else {
		log.Print("allowed_hosts config setting missing! Exiting!")
		os.Exit(1)
	}

	// TODO: make upload feature optional
	if allowed_push_hosts, ok := mainCfgSection["allowed_push_hosts"]; ok {
		allowedPushHosts = strings.Split(allowed_push_hosts, ",")
		Debugf("allowedPushHosts:", allowedPushHosts)
	} else {
		Debugf("No push hosts for check upload configured!")
	}

	// TODO: make upload feature optional
	if upload_dir, ok := mainCfgSection["upload_dir"]; ok {
		uploadDir = upload_dir
		Debugf("uploadDir:", uploadDir)
	} else {
		Debugf("No upload_dir configured!")
	}

	// TODO: make upload feature optional
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		log.Printf("upload_dir '%s' inaccessible", uploadDir)
		os.Exit(1)
	} else {
		if !strings.HasSuffix(uploadDir, "/") {
			uploadDir = uploadDir + "/"
		}
	}

	cfgCommands := &cfg.Commands
	// Names(): iterate over variables with undefined order and case
	Debugf("Found commands config settings:")
	for _, n := range cfgCommands.Names() {
		Debugf(n, " = ", *cfgCommands.Vals[cfgCommands.Idx(n)])
		commandsCfgSection[n] = *cfgCommands.Vals[cfgCommands.Idx(n)]
	}

	return ConfigSettings{mainCfgSection, commandsCfgSection}
}

// Debugf is a helper function for debug logging if mainCfgSection["debug"] is set
func Debugf(format string, args ...interface{}) {
	if mainCfgSection["debug"] != "0" {
		log.Print("DEBUG "+format, args)
	}
}

// randSeq returns a fixed length random string to identify each request in the log
// http://stackoverflow.com/a/22892986/682847
func randSeq() string {
	b := make([]rune, 8)
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b) + " "
}

func main() {

	var configFile = flag.String("config", "/etc/gorpe/gorpe.gcfg", "which config file to use at startup, defaults to /etc/gorpe/gorpe.gcfg")
	var foreGround = flag.Bool("fg", false, "if the log output should be sent to syslog or to STDOUT, defaults to false")
	var debugFlag = flag.Bool("debug", false, "log debug output, defaults to false")

	flag.Parse()

	if !*foreGround {
		// http://technosophos.com/2013/09/14/using-gos-built-logger-log-syslog.html
		// Configure logger to write to the syslog.
		logwriter, e := syslog.New(syslog.LOG_NOTICE, "gorpe")
		if e == nil {
			log.SetOutput(logwriter)
			log.Print("logging to syslog")
		}
	} else {
		log.Print("logging to STDOUT")
	}

	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		log.Printf("could not find config file: %s", *configFile)
		os.Exit(1)
	}

	if *debugFlag {
		log.Print("starting in DEBUG mode")
	}

	log.Print("using config file: ", *configFile)
	configSettings := readConfigfile(*configFile, *debugFlag)
	log.Print("found commands: ", configSettings.commands)

	// check if we need to generate certificates
	var certFilenames = map[string]string{
		"cert": mainCfgSection["certs_dir"] + "/cert.pem",
		"key":  mainCfgSection["certs_dir"] + "/key.pem",
	}

	for _, filename := range certFilenames {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			// generate certs
			Debugf("Certificate file: ", filename, " not found! Generating certificate...\n")
			gencerts.GenerateCert(certFilenames["cert"], certFilenames["key"])
			break
		} else {
			Debugf("Certificate file: ", filename, " found. Skipping certificate generation\n")
		}
	}

	http.HandleFunc("/", httpHandler)
	log.Printf("Listening on https://%s:%s/", configSettings.main["server_address"], configSettings.main["server_port"])
	//err := spdy.ListenAndServeSpdyOnly(":"+configSettings.main["server_port"], certFilenames["cert"], certFilenames["key"], nil)
	err := http.ListenAndServeTLS(":"+configSettings.main["server_port"], certFilenames["cert"], certFilenames["key"], nil)
	if err != nil {
		log.Fatal(err)
	}
}
