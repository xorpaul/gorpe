package main

import (
	"log"
	//"flag"
  "strings"
  "syscall"
  "os"
  //"reflect"
  "strconv"
  "os/exec"
	"net/http"
	//"html/template"

	"github.com/xorpaul/gencerts"
	//"github.com/SlyMarbo/spdy"
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
	log.Print("Request: ", req)
  ip := strings.Split(req.RemoteAddr,":")[0]
	log.Print("Incoming request from IP: ", ip)
	log.Print("Request path: ", req.URL.Path)

  reqParts := strings.Split(req.URL.Path[1:], "/")
  log.Printf("Request path parts are: %q", reqParts)
  command := reqParts[0]
  log.Print("Found command: ", command)
  cmdArguments := reqParts[1:len(reqParts)]
  log.Printf("Found command arguments: %q", cmdArguments)

	w.Header().Set("Content-Type", "text/plain")

  if _, ok := commandsCfgSection[command]; ok {
    argCount := strings.Count(commandsCfgSection[command], "$ARG$")
    log.Printf("Found %q command arguments in this command", len(cmdArguments))
    if argCount > len(cmdArguments) {
      w.Write([]byte("Too few command arguments!"))
    } else {
      cmdString := commandsCfgSection[command]
      log.Print("Got command from config: ", cmdString)
      for _, arg := range cmdArguments {
        // TODO: filter nasty chars
        if arg != "" {
          cmdString = strings.Replace(cmdString, "$ARG$", arg, 1)
          log.Printf("Replacing $ARG$ with %q results in %q", arg, cmdString)
        }
      }
      log.Print("Replacing arguments and executing: ", cmdString)
      cr := execCommand(cmdString)
      log.Print("Last char: ", cr.text[len(cr.text)-1])
      // Making sure that the check script output ends with a newline char
      if cr.text[len(cr.text)-1] != 10 {
        cr.text = append(cr.text, "\n"...)
      }
      output := append(cr.text, "Returncode: "...)
      output = strconv.AppendInt(output, int64(cr.returncode), 10)
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
      log.Print("err: ", err)
    } else {
      checkArguments = foobar
    }
    log.Printf("checkArguments are %q: ", checkArguments)
  }
  log.Print("checkScript: ", checkScript)
    log.Printf("checkArguments are %q: ", checkArguments)
  //if _, err := os.Stat(checkScript); os.IsNotExist(err) {
  //  return CheckResult{[]byte("UKNOWN: unknown output"), 3}
  //}
  //info, _ := os.Stat(checkScript)
  //mode := info.Mode()
  //log.Print("mode: ", mode & 0111)
  //log.Print("arg1: ", checkArguments[len(checkArguments)-1])

  out, err := exec.Command(checkScript, checkArguments...).Output()

  if err != nil {
    log.Print("err: ", err)
    if out == nil {
      out = []byte("UKNOWN: unknown output\n")
    }
    log.Print("out: ", string(out))
  } else {
    if msg, ok := err.(*exec.ExitError); ok { // there is error code
      returncode = msg.Sys().(syscall.WaitStatus).ExitStatus()
    } else {
      returncode = 0
    }
  }

	log.Print("Got output: ", string(out[:]))
	log.Print("Got return code: ", returncode)
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
  log.Print("Found main config settings:")
  for _, n := range cfgMain.Names() {
    log.Print(n, " = ", *cfgMain.Vals[cfgMain.Idx(n)])
    mainCfgSection[n] = *cfgMain.Vals[cfgMain.Idx(n)]
  }

  cfgCommands := &cfg.Commands
  // Names(): iterate over variables with undefined order and case
  log.Print("Found commands config settings:")
  for _, n := range cfgCommands.Names() {
    log.Print(n, " = ", *cfgCommands.Vals[cfgCommands.Idx(n)])
    commandsCfgSection[n] = *cfgCommands.Vals[cfgCommands.Idx(n)]
  }

  return ConfigSettings{mainCfgSection, commandsCfgSection}
}

func main() {

  configSettings := readConfigfile("config.gcfg")
  log.Print("found commands: ", configSettings.commands)


  // check if we need to generate certificates
  var certFilenames = map[string]string{
    "cert": "./cert.pem",
    "key": "./key.pem",
  }

  for _, filename := range certFilenames {
    if _, err := os.Stat(filename); os.IsNotExist(err) {
      // generate certs
      log.Print("Certificate file: ", filename, " not found! Generating certificate...\n")
      gencerts.GenerateCert()
      break
    } else {
      log.Print("Certificate file: ", filename, " found. Skipping certificate generation\n")
    }
  }


	http.HandleFunc("/", httpHandler)
	log.Printf("Listening on port %s. Go to https://%s:%s/", configSettings.main["server_port"], configSettings.main["server_address"], configSettings.main["server_port"])
  //err := spdy.ListenAndServeSpdyOnly(":"+configSettings.main["server_port"], certFilenames["cert"], certFilenames["key"], nil)
  //err := http.ListenAndServeTLS(":"+configSettings.main["server_port"], certFilenames["cert"], certFilenames["key"], nil)
	//if err != nil {
	//	log.Fatal(err)
	//}
}
