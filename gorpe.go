package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/xorpaul/gencerts"
	h "github.com/xorpaul/gohelper"
)

// keepAliveListener wraps a net.TCPListener and sets SO_KEEPALIVE +
// TCP_KEEPIDLE/TCP_KEEPINTVL on every accepted connection. The default
// listener Go gives us via http.Server.ListenAndServeTLS sets the period
// to 3 minutes, which means stateful firewalls / conntrack / NAT hops
// along the path can drop a flow that goes silent during a long-running
// command (gorpe buffers stdout+stderr until the child exits, so no
// app-layer bytes flow on the wire) before the kernel ever sends a probe.
type keepAliveListener struct {
	*net.TCPListener
	period time.Duration
}

func (ln keepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	if ln.period > 0 {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(ln.period)
	}
	return tc, nil
}

var start time.Time
var buildtime string
var buildversion string
var config = ConfigSettings{}
var keepAlivePeriod time.Duration
var requestCounter int
var forbiddenRequestCounter int
var failedRequestCounter int
var nastyMetachars = "|`&><'\"\\[]{};\n"

// ConfigSettings contains the key value pairs from the config file
type ConfigSettings struct {
	Main struct {
		ServerPort              int      `yaml:"server_port"`
		ServerAddress           string   `yaml:"server_address"`
		AllowedIPs              []string `yaml:"allowed_ips"`
		Debug                   int      `yaml:"debug"`
		CommandTimeout          int      `yaml:"command_timeout"`
		ConnectionTimeout       int      `yaml:"connection_timeout"`
		CertsDir                string   `yaml:"certs_dir"`
		VerifyClientCert        int      `yaml:"verify_client_cert"`
		CaFile                  string   `yaml:"ca_file"`
		ClientAuthDNs           []string `yaml:"client_auth_dns"`
		ClientAuthIssuer        []string `yaml:"client_auth_issuer"`
		ClientAuthIssuerCAFiles []string `yaml:"client_auth_issuer_ca_files"`
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

	if *versionFlag {
		fmt.Println("GORPE version", buildversion, "Build time:", buildtime, "UTC")
		os.Exit(0)
	}

	if !*foreGround {
		if err := setupSyslog(); err != nil {
			log.Printf("Failed to setup syslog, falling back to stdout: %v", err)
		}
	} else {
		log.Print("logging to STDOUT")
	}

	log.Println("started GORPE version", buildversion, "Build time:", buildtime, "UTC")

	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		log.Printf("could not find config file: %s", *configFile)
		os.Exit(1)
	}

	log.Print("using config file: ", *configFile)
	config = readConfigfile(*configFile, *debugFlag)
	log.Print("found commands: ", config.Commands)

	if len(config.Main.ClientAuthDNs) > 0 {
		if err := loadIssuerCACerts(config.Main.ClientAuthIssuerCAFiles); err != nil {
			log.Printf("failed to load client_auth_issuer_ca_files: %v", err)
			os.Exit(1)
		}
	}

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
			gencerts.GenerateCert(certFilenames["cert"], certFilenames["key"], false)
			break
		} else {
			h.Debugf("Certificate file: " + filename + " found. Skipping certificate generation\n")
		}
	}

	http.HandleFunc("/", httpHandler)

	// TLS stuff
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	if config.Main.VerifyClientCert == 1 {

		//Expect and verify client certificate against a CA cert
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

		caFile := config.Main.CaFile
		if _, err := os.Stat(caFile); os.IsNotExist(err) {
			log.Printf("could not find CA file: %s", caFile)
			os.Exit(1)
		} else {
			// Load CA cert
			caCert, err := os.ReadFile(caFile)
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
		Addr:         ":" + strconv.Itoa(config.Main.ServerPort),
		TLSConfig:    tlsConfig,
		WriteTimeout: time.Duration(config.Main.CommandTimeout+5) * time.Second,
		ReadTimeout:  time.Duration(config.Main.CommandTimeout+5) * time.Second,
		IdleTimeout:  time.Duration(config.Main.CommandTimeout+5) * time.Second,
	}

	keepAlivePeriod = time.Duration(config.Main.ConnectionTimeout) * time.Second
	if keepAlivePeriod <= 0 {
		// Fall back to a conservative default that defeats typical
		// stateful-firewall / NAT idle-flow timeouts (commonly 60-120s).
		keepAlivePeriod = 30 * time.Second
	}

	rawListener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatal(err)
	}
	listener := keepAliveListener{
		TCPListener: rawListener.(*net.TCPListener),
		period:      keepAlivePeriod,
	}

	log.Printf("Listening on https://%s:%d/ with TCP keepalive period %s",
		config.Main.ServerAddress, config.Main.ServerPort, keepAlivePeriod)
	err = server.ServeTLS(listener, certFilenames["cert"], certFilenames["key"])
	if err != nil {
		log.Fatal(err)
	}
}
