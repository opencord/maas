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
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"net/http"
)

type Config struct {
	Port            int    `default:"4243"`
	Listen          string `default:"0.0.0.0"`
	RoleSelectorURL string `default:"" envconfig:"role_selector_url"`
	DefaultRole     string `default:"compute-node" envconfig:"default_role"`
	Script          string `default:"do-ansible"`
	StorageURL      string `default:"memory:" envconfig:"storage_url"`
	LogLevel        string `default:"warning" envconfig:"LOG_LEVEL"`
	LogFormat       string `default:"text" envconfig:"LOG_FORMAT"`
}

type Context struct {
	config     Config
	storage    Storage
	workers    []Worker
	dispatcher *Dispatcher
}

var log = logrus.New()

func main() {
	context := &Context{}

	err := envconfig.Process("PROVISION", &(context.config))
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
	    Listen:          %s
	    Port:            %d
	    RoleSelectorURL: %s
	    DefaultRole:     %s
	    Script:          %s
	    StorageURL:      %s
	    Log Level:       %s
	    Log Format:      %s`,
		context.config.Listen, context.config.Port, context.config.RoleSelectorURL,
		context.config.DefaultRole, context.config.Script, context.config.StorageURL,
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
	context.dispatcher = NewDispatcher(5, context.storage)
	context.dispatcher.Start()

	http.ListenAndServe(fmt.Sprintf("%s:%d", context.config.Listen, context.config.Port), nil)
}
