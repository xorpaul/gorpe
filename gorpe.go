package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"log/syslog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/xorpaul/gencerts"
	h "github.com/xorpaul/gohelper"
)

var start time.Time
var buildtime string
var version string
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

func main() {
	start = time.Now()

	var (
		configFile  = flag.String("config", "/etc/gorpe/gorpe.yaml", "which config file to use at startup, defaults to /etc/gorpe/gorpe.yaml")
		foreGround  = flag.Bool("fg", false, "if the log output should be sent to syslog or to STDOUT, defaults to false")
		debugFlag   = flag.Bool("debug", false, "log debug output, defaults to false")
		versionFlag = flag.Bool("version", false, "show build time and version number")
	)

	flag.Parse()

	version = "2.0"
	if *versionFlag {
		fmt.Println("GORPE version", version, "Build time:", buildtime, "UTC")
		os.Exit(0)
	}

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

	log.Println("started GORPE version", version, "Build time:", buildtime, "UTC")

	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		log.Printf("could not find config file: %s", *configFile)
		os.Exit(1)
	}

	log.Print("using config file: ", *configFile)
	config = readConfigfile(*configFile, *debugFlag)
	log.Print("found commands: ", config.Commands)

	if *debugFlag || config.Main.Debug != 0 {
		h.Debug = true
		log.Print("starting in DEBUG mode")
	}
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
