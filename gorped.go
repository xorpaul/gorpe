package main

import (
	"flag"
	"log"
	"log/syslog"
  "strings"
  "syscall"
  "os"
  "strconv"
  "os/exec"
	"net/http"

	"github.com/xorpaul/gencerts"
  "code.google.com/p/gcfg"
  "github.com/kballard/go-shellquote"
)

var mainCfgSection = make(map[string]string)
var commandsCfgSection = make(map[string]string)

type ConfigSettings struct {
  main map[string]string
  commands map[string]string
}

type CheckResult struct {
  text []byte
  returncode int
}

func httpHandler(w http.ResponseWriter, req *http.Request) {
	Debugf("Request: ", req)
  ip := strings.Split(req.RemoteAddr,":")[0]
	Debugf("Incoming request from IP: ", ip)
	Debugf("Request path: ", req.URL.Path)

  reqParts := strings.Split(req.URL.Path[1:], "/")
  Debugf("Request path parts are: %q", reqParts)
  command := reqParts[0]
  Debugf("Found command: ", command)
  cmdArguments := reqParts[1:len(reqParts)]
  Debugf("Found command arguments: %q", cmdArguments)

	w.Header().Set("Content-Type", "text/plain")

  if _, ok := commandsCfgSection[command]; ok {
    argCount := strings.Count(commandsCfgSection[command], "$ARG$")
    Debugf("Found %q command arguments in this command", len(cmdArguments))
    if argCount > len(cmdArguments) {
      w.Write([]byte("Too few command arguments!"))
    } else {
      cmdString := commandsCfgSection[command]
      Debugf("Got command from config: ", cmdString)
      for _, arg := range cmdArguments {
        // TODO: filter nasty chars
        if arg != "" {
          cmdString = strings.Replace(cmdString, "$ARG$", arg, 1)
          Debugf("Replacing $ARG$ with %q results in %q", arg, cmdString)
        }
      }
      Debugf("Replacing arguments and executing: ", cmdString)
      cr := execCommand(cmdString)
      Debugf("Last char: ", cr.text[len(cr.text)-1])
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

}

func execCommand(cmdString string) CheckResult {
  returncode := 3
  parts := strings.SplitN(cmdString, " ", 2)
  checkScript := parts[0]
  checkArguments := []string{}
  if len(parts) > 1 {
    //checkArguments, err := shellquote.Split(strings.Join(parts[1:len(parts)], " "))
    foobar, err := shellquote.Split(parts[1])
    if err != nil {
      Debugf("err: ", err)
    } else {
      checkArguments = foobar
    }
    Debugf("checkArguments are %q: ", checkArguments)
  }
  Debugf("checkScript: ", checkScript)
    Debugf("checkArguments are %q: ", checkArguments)
  //if _, err := os.Stat(checkScript); os.IsNotExist(err) {
  //  return CheckResult{[]byte("UKNOWN: unknown output"), 3}
  //}
  //info, _ := os.Stat(checkScript)
  //mode := info.Mode()
  //Debugf("mode: ", mode & 0111)
  //Debugf("arg1: ", checkArguments[len(checkArguments)-1])

  out, err := exec.Command(checkScript, checkArguments...).Output()

  if err != nil {
    Debugf("err: ", err)
    if out == nil {
      out = []byte("UKNOWN: unknown output\n")
    }
    Debugf("out: ", string(out))
  } else {
    if msg, ok := err.(*exec.ExitError); ok { // there is error code
      returncode = msg.Sys().(syscall.WaitStatus).ExitStatus()
    } else {
      returncode = 0
    }
  }

	Debugf("Got output: ", string(out[:]))
	Debugf("Got return code: ", returncode)
  //return template.HTMLEscapeString(string(out))
  return CheckResult{out, returncode}
}

func readConfigfile(configFile string) ConfigSettings {
  //res := &struct{ Commands map[string] string }{}
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
    log.Print("There was an error parsing the configfile:", err)
	}

  cfgMain := &cfg.Main
  for _, n := range cfgMain.Names() {
    mainCfgSection[n] = *cfgMain.Vals[cfgMain.Idx(n)]
  }

  Debugf("Found main config settings:")
  for _, n := range cfgMain.Names() {
    Debugf(n, " = ", *cfgMain.Vals[cfgMain.Idx(n)])
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

func Debugf(format string, args ...interface{}) {
    if mainCfgSection["debug"] != "0" {
        log.Print("DEBUG " + format, args)
    }
}

func main() {

  // http://technosophos.com/2013/09/14/using-gos-built-logger-log-syslog.html
  // Configure logger to write to the syslog.
  logwriter, e := syslog.New(syslog.LOG_NOTICE, "gorpe")
  if e == nil {
    log.SetOutput(logwriter)
  }

  var configFile = flag.String("config", "/etc/gorpe/gorpe.gcfg", "which config file to use at startup, defaults to /etc/gorpe/gorpe.gcfg")

  if _, err := os.Stat(*configFile); os.IsNotExist(err) {
    log.Printf("could not find config file: %s", *configFile)
    os.Exit(1)
  }

  log.Print("using config file: ", *configFile)
  configSettings := readConfigfile(*configFile)
  log.Print("found commands: ", configSettings.commands)


  // check if we need to generate certificates
  var certFilenames = map[string]string{
    "cert": mainCfgSection["certs_dir"] + "/cert.pem",
    "key": mainCfgSection["certs_dir"] + "/key.pem",
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
