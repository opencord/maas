// Copyright 2016 Open Networking Laboratory
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
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

	maas "github.com/juju/gomaasapi"
)

const (
	// defaultFilter specifies the default filter to use when none is specified
	defaultFilter = `{
	  "hosts" : {
	    "include" : [ ".*" ],
		"exclude" : []
	  },
	  "zones" : {
	    "include" : ["default"],
		"exclude" : []
           }
	}`
	defaultMapping = "{}"
)

type Config struct {
	PowerHelperUser   string `default:"cord" envconfig:"POWER_HELPER_USER"`
	PowerHelperHost   string `default:"127.0.0.1" envconfig:"POWER_HELPER_HOST"`
	PowerHelperScript string `default:"" envconfig:"POWER_HELPER_SCRIPT"`
	ProvisionUrl      string `default:"" envconfig:"PROVISION_URL"`
	ProvisionTtl      string `default:"1h" envconfig:"PROVISION_TTL"`
	LogLevel          string `default:"warning" envconfig:"LOG_LEVEL"`
	LogFormat         string `default:"text" envconfig:"LOG_FORMAT"`
}

var apiKey = flag.String("apikey", "", "key with which to access MAAS server")
var maasURL = flag.String("maas", "http://localhost/MAAS", "url over which to access MAAS")
var apiVersion = flag.String("apiVersion", "1.0", "version of the API to access")
var queryPeriod = flag.String("period", "15s", "frequency the MAAS service is polled for node states")
var preview = flag.Bool("preview", false, "displays the action that would be taken, but does not do the action, in this mode the nodes are processed only once")
var mappings = flag.String("mappings", "{}", "the mac to name mappings")
var always = flag.Bool("always-rename", true, "attempt to rename at every stage of workflow")
var filterSpec = flag.String("filter", strings.Map(func(r rune) rune {
	if unicode.IsSpace(r) {
		return -1
	}
	return r
}, defaultFilter), "constrain by hostname what will be automated")

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

func main() {

	flag.Parse()
	config := Config{}
	err := envconfig.Process("AUTOMATION", &config)
	if err != nil {
		log.Fatalf("Unable to parse configuration options : %s", err)
	}

	options := ProcessingOptions{
		Preview:         *preview,
		AlwaysRename:    *always,
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
	if len(*filterSpec) > 0 {
		if (*filterSpec)[0] == '@' {
			name := os.ExpandEnv((*filterSpec)[1:])
			file, err := os.OpenFile(name, os.O_RDONLY, 0)
			checkError(err, "unable to open file '%s' to load the filter : %s", name, err)
			decoder := json.NewDecoder(file)
			err = decoder.Decode(&options.Filter)
			checkError(err, "unable to parse filter configuration from file '%s' : %s", name, err)
		} else {
			err := json.Unmarshal([]byte(*filterSpec), &options.Filter)
			checkError(err, "unable to parse filter specification: '%s' : %s", *filterSpec, err)
		}
	} else {
		err := json.Unmarshal([]byte(defaultFilter), &options.Filter)
		checkError(err, "unable to parse default filter specificiation: '%s' : %s", defaultFilter, err)
	}

	// Determine the mac to name mapping, this can either be specified on the the command
	// line as a value or a file reference. If none is specified the default
	// will be used
	if len(*mappings) > 0 {
		if (*mappings)[0] == '@' {
			name := os.ExpandEnv((*mappings)[1:])
			file, err := os.OpenFile(name, os.O_RDONLY, 0)
			checkError(err, "unable to open file '%s' to load the mac name mapping : %s", name, err)
			decoder := json.NewDecoder(file)
			err = decoder.Decode(&options.Mappings)
			checkError(err, "unable to parse filter configuration from file '%s' : %s", name, err)
		} else {
			err := json.Unmarshal([]byte(*mappings), &options.Mappings)
			checkError(err, "unable to parse mac name mapping: '%s' : %s", *mappings, err)
		}
	} else {
		err := json.Unmarshal([]byte(defaultMapping), &options.Mappings)
		checkError(err, "unable to parse default mac name mappings: '%s' : %s", defaultMapping, err)
	}

	// Verify the specified period for queries can be converted into a Go duration
	period, err := time.ParseDuration(*queryPeriod)
	checkError(err, "unable to parse specified query period duration: '%s': %s", queryPeriod, err)

	log.Infof(`Configuration:
	    MAAS URL:            %s
	    MAAS API Version:    %s
	    MAAS Query Interval: %s
	    Node Filter:         %s
	    Node Name Mappings:  %s
	    Preview:             %v
	    Always Rename:       %v
	    Provision URL:       %s
	    Provision TTL:       %s
	    Power Helper:        %s
	    Power Helper User:   %s
	    Power Helper Host:   %s
	    Log Level:           %s
	    Log Format:		 %s`,
		*maasURL, *apiVersion, *queryPeriod, *filterSpec, *mappings, options.Preview,
		options.AlwaysRename, options.ProvisionURL, options.ProvisionTTL,
		options.PowerHelper, options.PowerHelperUser, options.PowerHelperHost, config.LogLevel, config.LogFormat)

	authClient, err := maas.NewAuthenticatedClient(*maasURL, *apiKey, *apiVersion)
	if err != nil {
		checkError(err, "Unable to use specified client key, '%s', to authenticate to the MAAS server: %s", *apiKey, err)
	}

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

	if !(*preview) {
		// Create a ticker and fetch and process the nodes every "period"
		ticker := time.NewTicker(period)
		for t := range ticker.C {
			log.Infof("query server at %s", t)
			nodes, _ := fetchNodes(client)
			ProcessAll(client, nodes, options)
		}
	}
}
