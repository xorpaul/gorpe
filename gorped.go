package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"log"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xorpaul/gencerts"
	h "github.com/xorpaul/gohelper"
)

var start time.Time
var buildtime string
var config = ConfigSettings{}
var requestCounter int
var forbiddenRequestCounter int
var failedRequestCounter int
var nastyMetachars = "|`&><'\"\\[]{};\n"

// ConfigSettings contains the key value pairs from the config file
type ConfigSettings struct {
	Main struct {
		ServerPort        int      `yaml:"server_port"`
		ServerAddress     string   `yaml:"server_address"`
		AllowedHosts      []string `yaml:"allowed_hosts"`
		Debug             int      `yaml:"debug"`
		CommandTimeout    int      `yaml:"command_timeout"`
		ConnectionTimeout int      `yaml:"connection_timeout"`
		CertsDir          string   `yaml:"certs_dir"`
		VerifyClientCert  int      `yaml:"verify_client_cert"`
		CaFile            string   `yaml:"ca_file"`
	} `yaml:"main"`
	Commands map[string]string `yaml:"commands"`
}

// checkResult represent the result of an check script
type checkResult struct {
	text       string
	returncode int
}

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
			sslText := "SSL Client Verify disabled"
			if config.Main.VerifyClientCert == 1 {
				sslText = "SSL Client Verify enabled"
			}
			checkResult{"GORPE version 1.4 HTTP/2 " + sslText + " Build time: " + buildtime + perfData, 0}.Exit(w)
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
				cr := h.ExecuteCommand(cmdString, config.Main.CommandTimeout, true)
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

func main() {
	start = time.Now()

	var (
		configFile = flag.String("config", "/etc/gorpe/gorpe.yaml", "which config file to use at startup, defaults to /etc/gorpe/gorpe.yaml")
		foreGround = flag.Bool("fg", false, "if the log output should be sent to syslog or to STDOUT, defaults to false")
		debugFlag  = flag.Bool("debug", false, "log debug output, defaults to false")
	)

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
		h.Debug = true
		log.Print("starting in DEBUG mode")
	}

	log.Print("using config file: ", *configFile)
	config = readConfigfile(*configFile, *debugFlag)
	log.Print("found commands: ", config.Commands)

	// check if we need to generate certificates
	var certFilenames = map[string]string{
		"cert": filepath.Join(config.Main.CertsDir, "cert.pem"),
		"key":  filepath.Join(config.Main.CertsDir, "key.pem"),
	}

	for _, filename := range certFilenames {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			// generate certs
			h.Debugf("Certificate file: " + filename + " not found! Generating certificate...\n")
			gencerts.GenerateCert(certFilenames["cert"], certFilenames["key"])
			break
		} else {
			h.Debugf("Certificate file: " + filename + " found. Skipping certificate generation\n")
		}
	}

	http.HandleFunc("/", httpHandler)

	// TLS stuff
	tlsConfig := &tls.Config{}
	//Use only TLS v1.2
	tlsConfig.MinVersion = tls.VersionTLS12

	if config.Main.VerifyClientCert == 1 {

		//Expect and verify client certificate against a CA cert
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

		caFile := config.Main.CaFile
		if _, err := os.Stat(caFile); os.IsNotExist(err) {
			log.Printf("could not find CA file: %s", caFile)
			os.Exit(1)
		} else {
			// Load CA cert
			caCert, err := ioutil.ReadFile(caFile)
			if err != nil {
				log.Fatal(err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.ClientCAs = caCertPool
			log.Print("Expecting and verifing client certificate against " + caFile)
		}
	}
	server := &http.Server{
		Addr:      ":" + strconv.Itoa(config.Main.ServerPort),
		TLSConfig: tlsConfig,
	}

	log.Print("Listening on https://" + config.Main.ServerAddress + ":" + strconv.Itoa(config.Main.ServerPort) + "/")
	err := server.ListenAndServeTLS(certFilenames["cert"], certFilenames["key"])
	if err != nil {
		log.Fatal(err)
	}
}
