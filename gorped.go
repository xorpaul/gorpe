package main

import (
	"log"
	"flag"
  "strings"
  "syscall"
  "os"
  //"reflect"
  "strconv"
  "os/exec"
	"net/http"
	//"html/template"

	"github.com/xorpaul/gencerts"
	"github.com/SlyMarbo/spdy"
  "code.google.com/p/gcfg"

)

var commands = make(map[string]string)

type CheckResult struct {
  text []byte
  returncode int
}

func httpHandler(w http.ResponseWriter, req *http.Request) {
	log.Print("Request: ", req)
	log.Print("Request path: ", req.URL.Path)

  reqParts := strings.Split(req.URL.Path[1:], "/")
  log.Printf("Request path parts are: %q", reqParts)
  command := reqParts[0]
  log.Print("Found command: ", command)
  cmdArguments := reqParts[1:len(reqParts)]
  log.Printf("Found command arguments: %q", cmdArguments)

	w.Header().Set("Content-Type", "text/plain")

  if _, ok := commands[command]; ok {
    argCount := strings.Count(commands[command], "$ARG$")
    if argCount > len(cmdArguments) {
      w.Write([]byte("Too few command arguments!"))
    } else {
      cmdString, cmdStringFromConfig := "", commands[command]
      log.Print("Got command from config: ", cmdStringFromConfig)
      for _, arg := range cmdArguments {
        // TODO: filter nasty chars
        if arg != "" {
          cmdString = strings.Replace(cmdStringFromConfig, "$ARG$", arg, 1)
          log.Printf("Replacing $ARG$ with %q results in %q", arg, cmdString)
          cmdStringFromConfig = cmdString
        }
      }
      log.Print("Replacing arguments and executing: ", cmdString)
      cr := execCommand(cmdString)
      output := append(cr.text, "Returncode: "...)
      output = strconv.AppendInt(output, int64(cr.returncode), 10)
      //log.Print("rc: ", cr.returncode)
      w.Write(output)
      //w.Write([]byte(string(cr.returncode)))
    }
  } else {
    //w.Write([]byte("Command ", uri, "not found!"))
    w.Write([]byte("Command not found!"))
  }

}

func execCommand(cmdString string) CheckResult {
  returncode := 3
  parts := strings.Fields(cmdString)
  head := parts[0]
  parts = parts[1:len(parts)]
	//log.Print("head: ", head)
	//log.Printf("parts are %q: ", parts)
  cmd := exec.Command(head, parts...)
  out, err := cmd.Output()
  //if err != nil && err != "exit status 3" {
  //  log.Print("err: ", err)
  //  log.Print("out: ", string(out))
  //  out = []byte("unknown")
  //} else {
  if msg, ok := err.(*exec.ExitError); ok { // there is error code
    returncode = msg.Sys().(syscall.WaitStatus).ExitStatus()
  } else {
    returncode = 0
  }
  //}

	log.Print("Got output: ", string(out[:]))
	log.Print("Got return code: ", returncode)
  //return template.HTMLEscapeString(string(out))
  return CheckResult{out, returncode}
}

func readConfigfile(configFile string) map[string]string {
  //res := &struct{ Commands map[string] string }{}
  var cfg = &struct {
    Commands struct {
      gcfg.Idxer
      Vals map[gcfg.Idx]*string
    }
  }{}
  if err := gcfg.ReadFileInto(cfg, configFile); err != nil {
		panic(err)
	}

  config := &cfg.Commands
  //var commands = make(map[string]string)
  // Names(): iterate over variables with undefined order and case
	for _, n := range config.Names() {
		log.Print(n, " and ", *config.Vals[config.Idx(n)])
    commands[n] = *config.Vals[config.Idx(n)]
	}

  return commands
}

func main() {

  var (
    flagListenIp   = flag.String("listenIp", "", "IP to listen for incoming connections.")
    flagListenPort = flag.String("listenPort", "10443", "Port to listen for incoming connections.")
  )

  var commands = readConfigfile("config.gcfg")
  log.Print("found commands: ", commands)


  // config settings
  var config = map[string]string{
    "listenIp": *flagListenIp,
    "listenPort": *flagListenPort,
  }
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
//	http.HandleFunc("/warning", execTest("warning"))
//	http.HandleFunc("/critical", execTest("critical"))
//	http.HandleFunc("/unknown", execTest("unknown"))
	log.Printf("Listening on port %s. Go to https://127.0.0.1:%s/", config["listenPort"], config["listenPort"])
  err := spdy.ListenAndServeSpdyOnly(":"+config["listenPort"], certFilenames["cert"], certFilenames["key"], nil)
	if err != nil {
		log.Fatal(err)
	}
}
