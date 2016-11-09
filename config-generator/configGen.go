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
	"net/http"

	"github.com/Sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port       int    `default:"1337"`
	Listen     string `default:"0.0.0.0"`
	Controller string `default:"http://%s:%s@127.0.0.1:8181"`
	Username   string `default:"karaf"`
	Password   string `default:"karaf"`
	LogLevel   string `default:"warning" envconfig:"LOG_LEVEL"`
	LogFormat  string `default:"text" envconfig:"LOG_FORMAT"`

	connect string
}

var c Config
var log = logrus.New()

func main() {

	err := envconfig.Process("CONFIGGEN", &c)
	if err != nil {
		log.Fatalf("[ERROR] Unable to parse configuration options : %s", err)
	}

	switch c.LogFormat {
	case "json":
		log.Formatter = &logrus.JSONFormatter{}
	default:
		log.Formatter = &logrus.TextFormatter{
			FullTimestamp: true,
			ForceColors:   true,
		}
	}

	level, err := logrus.ParseLevel(c.LogLevel)
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
		c.Listen, c.Port, c.Controller,
		c.Username, c.Password,
		c.LogLevel, c.LogFormat)

	router := mux.NewRouter()
	router.HandleFunc("/config/", c.configGenHandler).Methods("POST")
	http.Handle("/", router)

	c.connect = fmt.Sprintf(c.Controller, c.Username, c.Password)

	panic(http.ListenAndServe(fmt.Sprintf(":%d", c.Port), nil))
}
