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
	"flag"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"net/http"
	"os"
)

const appName = "PROVISION"

type Config struct {
	Port            int    `default:"4243" desc:"port on which to listen for requests"`
	Listen          string `default:"0.0.0.0" desc:"IP on which to listen for requests"`
	RoleSelectorURL string `default:"" envconfig:"ROLE_SELECTOR_URL" desc:"connection string to query role for device"`
	DefaultRole     string `default:"compute-node" envconfig:"DEFAULT_ROLE" desc:"default role for device"`
	Script          string `default:"do-ansible" desc:"default script to execute to provision device"`
	StorageURL      string `default:"memory:" envconfig:"STORAGE_URL" desc:"connection string to persistence implementation"`
	NumberOfWorkers int    `default:"5" envconfig:"NUMBER_OF_WORKERS" desc:"number of concurrent provisioning workers"`
	LogLevel        string `default:"warning" envconfig:"LOG_LEVEL" desc:"detail level for logging"`
	LogFormat       string `default:"text" envconfig:"LOG_FORMAT" desc:"log output format, text or json"`
}

type Context struct {
	config     Config
	storage    Storage
	workers    []Worker
	dispatcher *Dispatcher
}

var log = logrus.New()
var appFlags = flag.NewFlagSet("", flag.ContinueOnError)

func main() {
	context := &Context{}

	appFlags.Usage = func() {
		envconfig.Usage(appName, &(context.config))
	}
	if err := appFlags.Parse(os.Args[1:]); err != nil {
		if err != flag.ErrHelp {
			os.Exit(1)
		} else {
			return
		}
	}
	err := envconfig.Process(appName, &(context.config))
	if err != nil {
		log.Fatalf("[ERRO] Unable to parse configuration options : %s", err)
	}

	switch context.config.LogFormat {
	case "json":
		log.Formatter = &logrus.JSONFormatter{}
	default:
		log.Formatter = &logrus.TextFormatter{
			FullTimestamp: true,
			ForceColors:   true,
		}
	}

	level, err := logrus.ParseLevel(context.config.LogLevel)
	if err != nil {
		level = logrus.WarnLevel
	}
	log.Level = level

	log.Infof(`Configuration:
	    LISTEN:             %s
	    PORT:               %d
	    ROLE_SELECTION_URL: %s
	    DEFAULT_ROLE:       %s
	    SCRIPT:             %s
	    STORAGE_URL:        %s
	    NUMBER_OF_WORERS:   %d
	    LOG_LEVEL:          %s
	    LOG_FORMAT:         %s`,
		context.config.Listen, context.config.Port, context.config.RoleSelectorURL,
		context.config.DefaultRole, context.config.Script, context.config.StorageURL,
		context.config.NumberOfWorkers,
		context.config.LogLevel, context.config.LogFormat)

	context.storage, err = NewStorage(context.config.StorageURL)
	if err != nil {
		log.Fatalf("[error] Unable to connect to specified storage '%s' : %s",
			context.config.StorageURL, err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/provision/", context.ProvisionRequestHandler).Methods("POST")
	router.HandleFunc("/provision/", context.ListRequestsHandler).Methods("GET")
	router.HandleFunc("/provision/{nodeid}", context.QueryStatusHandler).Methods("GET")
	router.HandleFunc("/provision/{nodeid}", context.DeleteStatusHandler).Methods("DELETE")
	http.Handle("/", router)

	// Start the dispatcher and workers
	context.dispatcher = NewDispatcher(context.config.NumberOfWorkers, context.storage)
	context.dispatcher.Start()

	http.ListenAndServe(fmt.Sprintf("%s:%d", context.config.Listen, context.config.Port), nil)
}
