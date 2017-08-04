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
	"net/http"
	"os"

	"github.com/Sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
)

const appName = "CONFIGGEN"

type Config struct {
	Port       int    `default:"1337" desc:"port on which to listen for requests"`
	Listen     string `default:"0.0.0.0" desc:"IP address on which to listen for requests"`
	Controller string `default:"http://%s:%s@127.0.0.1:8181" desc:"connection string with which to connect to ONOS"`
	Username   string `default:"karaf" desc:"username with which to connect to ONOS"`
	Password   string `default:"karaf" desc:"password with which to connect to ONOS"`
	LogLevel   string `default:"warning" envconfig:"LOG_LEVEL" desc:"detail level for logging"`
	LogFormat  string `default:"text" envconfig:"LOG_FORMAT" desc:"log output format, text or json"`

	connect string
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

	err := envconfig.Process("CONFIGGEN", &config)
	if err != nil {
		log.Fatalf("[ERROR] Unable to parse configuration options : %s", err)
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

	log.Infof(`Configuration:
        LISTEN:     %s
        PORT:       %d
        CONTROLLER: %s
        USERNAME:   %s
        PASSWORD:   %s
        LOG_LEVEL:  %s
        LOG_FORMAT: %s`,
		config.Listen, config.Port, config.Controller,
		config.Username, config.Password,
		config.LogLevel, config.LogFormat)

	router := mux.NewRouter()
	router.HandleFunc("/config/", config.configGenHandler).Methods("POST")
	http.Handle("/", router)

	config.connect = fmt.Sprintf(config.Controller, config.Username, config.Password)

	panic(http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil))
}
