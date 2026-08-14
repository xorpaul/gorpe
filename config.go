package main

import (
	"log"
	"os"

	h "github.com/xorpaul/gohelper"
	"gopkg.in/yaml.v2"
)

// readConfigfile creates the MainCfgSection and commandsCfgSection structs
// from the gorpe config file
func readConfigfile(configFile string, debugFlag bool) ConfigSettings {
	data, err := os.ReadFile(configFile)
	if err != nil {
		h.Fatalf("readConfigfile(): There was an error parsing the config file " + configFile + ": " + err.Error())
	}

	var config ConfigSettings
	err = yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		h.Fatalf("In config file " + configFile + ": YAML unmarshal error: " + err.Error())
	}

	//fmt.Print("config: ")
	//fmt.Printf("%+v\n", config.Main)

	if debugFlag {
		if config.Main.Debug == 0 {
			log.Print("overriding debug config file setting, because debug flag was set")
		}
		config.Main.Debug = 1
	}

	h.Debugf("Found Main config settings:")
	if config.Main.Debug == 1 {
		log.Printf("%+v\n", config.Main)
	}

	if len(config.Main.AllowedIPs) <= 0 {
		h.Fatalf("allowed_ips config setting missing! Exiting!")
	}

	if len(config.Main.ClientAuthDNs) > 0 {
		if config.Main.VerifyClientCert != 1 {
			h.Fatalf("client_auth_dns requires verify_client_cert: 1")
		}
		if len(config.Main.ClientAuthIssuerCAFiles) == 0 {
			h.Fatalf("client_auth_dns requires client_auth_issuer_ca_files to be set")
		}
	}

	h.Debugf("Found commands config settings:")
	if config.Main.Debug == 1 {
		log.Printf("%+v\n", config.Commands)
	}

	return ConfigSettings{config.Main, config.Commands}
}
