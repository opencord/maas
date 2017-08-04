// Copyright 2016 Open Networking Foundation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"encoding/json"
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/kelseyhightower/envconfig"
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"time"

	maas "github.com/juju/gomaasapi"
)

const appName = "AUTOMATION"

type Config struct {
	PowerHelperUser   string        `default:"cord" envconfig:"POWER_HELPER_USER" desc:"user when integrating with virtual box power mgmt"`
	PowerHelperHost   string        `default:"127.0.0.1" envconfig:"POWER_HELPER_HOST" desc:"virtual box host"`
	PowerHelperScript string        `default:"" envconfig:"POWER_HELPER_SCRIPT" desc:"script for virtual box power mgmt support"`
	ProvisionUrl      string        `default:"" envconfig:"PROVISION_URL" desc:"connection string to connect to provisioner uservice"`
	ProvisionTtl      string        `default:"1h" envconfig:"PROVISION_TTL" desc:"duration to wait for a provisioning request to complete, before considered a failure"`
	LogLevel          string        `default:"warning" envconfig:"LOG_LEVEL" desc:"detail level for logging"`
	LogFormat         string        `default:"text" envconfig:"LOG_FORMAT" desc:"log output format, text or json"`
	ApiKey            string        `envconfig:"MAAS_API_KEY" required:"true" desc:"API key to access MAAS server"`
	ApiKeyFile        string        `default:"/secrets/maas_api_key" envconfig:"MAAS_API_KEY_FILE" desc:"file to hold the secret"`
	ShowApiKey        bool          `default:"false" envconfig:"MAAS_SHOW_API_KEY" desc:"Show API in clear text in logs"`
	MaasUrl           string        `default:"http://localhost/MAAS" envconfig:"MAAS_URL" desc:"URL to access MAAS server"`
	ApiVersion        string        `default:"1.0" envconfig:"MAAS_API_VERSION" desc:"API version to use with MAAS server"`
	QueryInterval     time.Duration `default:"15s" envconfig:"MAAS_QUERY_INTERVAL" desc:"frequency to query MAAS service for nodes"`
	PreviewOnly       bool          `default:"false" envconfig:"PREVIEW_ONLY" desc:"display actions that would be taken, but don't execute them"`
	AlwaysRename      bool          `default:"true" envconfig:"ALWAYS_RENAME" desc:"attempt to rename hosts at every stage or workflow"`
	Mappings          string        `default:"{}" envconfig:"MAC_TO_NAME_MAPPINGS" desc:"custom MAC address to host name mappings"`
	FilterSpec        string        `default:"{\"hosts\":{\"include\":[\".*\"]},\"zones\":{\"include\":[\"default\"]}}" envconfig:"HOST_FILTER_SPEC" desc:"constrain hosts that are automated"`
}

// checkError if the given err is not nil, then fatally log the message, else
// return false.
func checkError(err error, message string, v ...interface{}) bool {
	if err != nil {
		log.Fatalf(message, v...)
	}
	return false
}

// checkWarn if the given err is not nil, then log the message as a warning and
// return true, else return false.
func checkWarn(err error, message string, v ...interface{}) bool {
	if err != nil {
		log.Warningf(message, v...)
		return true
	}
	return false
}

// fetchNodes do a HTTP GET to the MAAS server to query all the nodes
func fetchNodes(client *maas.MAASObject) ([]MaasNode, error) {
	nodeListing := client.GetSubObject("nodes")
	listNodeObjects, err := nodeListing.CallGet("list", url.Values{})
	if checkWarn(err, "unable to get the list of all nodes: %s", err) {
		return nil, err
	}
	listNodes, err := listNodeObjects.GetArray()
	if checkWarn(err, "unable to get the node objects for the list: %s", err) {
		return nil, err
	}

	var nodes = make([]MaasNode, len(listNodes))
	for index, nodeObj := range listNodes {
		node, err := nodeObj.GetMAASObject()
		if !checkWarn(err, "unable to retrieve object for node: %s", err) {
			nodes[index] = MaasNode{node}
		}
	}
	return nodes, nil
}

var log = logrus.New()
var appFlags = flag.NewFlagSet("", flag.ContinueOnError)

func main() {

	config := Config{}
	appFlags.Usage = func() {
		envconfig.Usage(appName, &config)
	}
	if err := appFlags.Parse(os.Args[1:]); err != nil {
		if err != flag.ErrHelp {
			os.Exit(1)
		} else {
			return
		}
	}

	err := envconfig.Process(appName, &config)

	if err != nil {
		log.Fatalf("Unable to parse configuration options : %s", err)
	}

	options := ProcessingOptions{
		Preview:         config.PreviewOnly,
		AlwaysRename:    config.AlwaysRename,
		Provisioner:     NewProvisioner(&ProvisionerConfig{Url: config.ProvisionUrl}),
		ProvisionURL:    config.ProvisionUrl,
		PowerHelper:     config.PowerHelperScript,
		PowerHelperUser: config.PowerHelperUser,
		PowerHelperHost: config.PowerHelperHost,
	}

	switch config.LogFormat {
	case "json":
		log.Formatter = &logrus.JSONFormatter{}
	default:
		log.Formatter = &logrus.TextFormatter{
			FullTimestamp: true,
			ForceColors:   true,
		}
	}

	level, err := logrus.ParseLevel(config.LogLevel)
	if err != nil {
		level = logrus.WarnLevel
	}
	log.Level = level

	options.ProvisionTTL, err = time.ParseDuration(config.ProvisionTtl)
	checkError(err, "unable to parse specified duration of '%s' : %s", err)

	// Determine the filter, this can either be specified on the the command
	// line as a value or a file reference. If none is specified the default
	// will be used
	if len(config.FilterSpec) > 0 {
		if config.FilterSpec[0] == '@' {
			name := os.ExpandEnv((config.FilterSpec)[1:])
			file, err := os.OpenFile(name, os.O_RDONLY, 0)
			checkError(err, "unable to open file '%s' to load the filter : %s", name, err)
			decoder := json.NewDecoder(file)
			err = decoder.Decode(&options.Filter)
			checkError(err, "unable to parse filter configuration from file '%s' : %s", name, err)
		} else {
			err := json.Unmarshal([]byte(config.FilterSpec), &options.Filter)
			checkError(err, "unable to parse filter specification: '%s' : %s", config.FilterSpec, err)
		}
	}

	// Determine the mac to name mapping, this can either be specified on the the command
	// line as a value or a file reference. If none is specified the default
	// will be used
	if len(config.Mappings) > 0 {
		if config.Mappings[0] == '@' {
			name := os.ExpandEnv(config.Mappings[1:])
			file, err := os.OpenFile(name, os.O_RDONLY, 0)
			checkError(err, "unable to open file '%s' to load the mac name mapping : %s", name, err)
			decoder := json.NewDecoder(file)
			err = decoder.Decode(&options.Mappings)
			checkError(err, "unable to parse filter configuration from file '%s' : %s", name, err)
		} else {
			err := json.Unmarshal([]byte(config.Mappings), &options.Mappings)
			checkError(err, "unable to parse mac name mapping: '%s' : %s", config.Mappings, err)
		}
	}

	// Get human readable strings for config output
	mappingsAsJson, err := json.Marshal(options.Mappings)
	checkError(err, "Unable to marshal MAC to hostname mappings to JSON : %s", err)
	mappingsPrefix := ""

	if len(config.Mappings) > 0 && config.Mappings[0] == '@' {
		mappingsPrefix = "[" + config.Mappings + "]"
	}

	filterAsJson, err := json.Marshal(options.Filter)
	checkError(err, "Unable to marshal host filter to JSON : %s", err)
	filterPrefix := ""
	if len(config.FilterSpec) > 0 && config.FilterSpec[0] == '@' {
		filterPrefix = "[" + config.FilterSpec + "]"
	}

	re := regexp.MustCompile("[^:]")
	pubKey := config.ApiKey
	if !config.ShowApiKey {
		pubKey = re.ReplaceAllString(config.ApiKey, "X")
	}

	log.Infof(`Configuration:
            POWER_HELPER_USER:    %s
	    POWER_HELPER_HOST:    %s
	    POWER_HELPER_SCRIPT:  %s
	    PROVISION_URL:        %s
	    PROVISION_TTL:        %s
	    MAAS_URL:             %s
	    MAAS_SHOW_API_KEY:    %t
	    MAAS_API_KEY:         %s
	    MAAS_API_KEY_FILE:    %s
	    MAAS_API_VERSION:     %s
	    MAAS_QUERY_INTERVAL:  %s
	    HOST_FILTER_SPEC:     %+v
	    MAC_TO_NAME_MAPPINGS: %+v
	    PREVIEW_ONLY:         %t
	    ALWAYS_RENAME:        %t
	    LOG_LEVEL:            %s
	    LOG_FORMAT:		  %s`,
		config.PowerHelperUser, config.PowerHelperHost, config.PowerHelperScript,
		config.ProvisionUrl, config.ProvisionTtl,
		config.MaasUrl, config.ShowApiKey,
		pubKey, config.ApiKeyFile, config.ApiVersion, config.QueryInterval,
		filterPrefix+string(filterAsJson), mappingsPrefix+string(mappingsAsJson),
		config.PreviewOnly, config.AlwaysRename,
		config.LogLevel, config.LogFormat)

	// Attempt to load the API key from a file if it was not set via the environment
	// and if the file exists
	if config.ApiKey == "" {
		log.Debugf("Attempting to read MAAS API key from file '%s', because it was not set via environment", config.ApiKeyFile)
		keyBytes, err := ioutil.ReadFile(config.ApiKeyFile)
		if err != nil {
			log.Warnf("Failed to read MAAS API key from file '%s', was the file mounted as a volume? : %s ",
				config.ApiKeyFile, err)
		} else {
			config.ApiKey = string(keyBytes)
			if config.ShowApiKey {
				pubKey = config.ApiKey
			} else {
				pubKey = re.ReplaceAllString(config.ApiKey, "X")
			}
		}
	}

	authClient, err := maas.NewAuthenticatedClient(config.MaasUrl, config.ApiKey, config.ApiVersion)
	checkError(err, "Unable to use specified client key, '%s', to authenticate to the MAAS server: %s",
		pubKey, err)

	// Create an object through which we will communicate with MAAS
	client := maas.NewMAAS(*authClient)

	// This utility essentially polls the MAAS server for node state and
	// process the node to the next state. This is done by kicking off the
	// process every specified duration. This means that the first processing of
	// nodes will have "period" in the future. This is really not the behavior
	// we want, we really want, do it now, and then do the next one in "period".
	// So, the code does one now.
	nodes, _ := fetchNodes(client)
	ProcessAll(client, nodes, options)

	if !(config.PreviewOnly) {
		// Create a ticker and fetch and process the nodes every "period"
		for {
			log.Infof("query server at %s", time.Now())
			nodes, _ := fetchNodes(client)
			ProcessAll(client, nodes, options)

			// Sleep for the Interval and then process again.
			time.Sleep(config.QueryInterval)
		}
	}
}
