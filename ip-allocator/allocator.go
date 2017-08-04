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

const appName = "ALLOCATE"

type Config struct {
	Port      int    `default:"4242" desc:"port on which to listen for requests"`
	Listen    string `default:"0.0.0.0" desc:"IP on which to listen for requests"`
	Network   string `default:"10.0.0.0/24" desc:"subnet to allocate via requests"`
	RangeLow  string `default:"10.0.0.2" envconfig:"RANGE_LOW" desc:"low value in range to allocate"`
	RangeHigh string `default:"10.0.0.253" envconfig:"RANGE_HIGH" desc:"high value in range to allocate"`
	LogLevel  string `default:"warning" envconfig:"LOG_LEVEL" desc:"detail level for logging"`
	LogFormat string `default:"text" envconfig:"LOG_FORMAT" desc:"log output format, text or json"`
}

type Context struct {
	storage Storage
}

var log = logrus.New()
var appFlags = flag.NewFlagSet("", flag.ContinueOnError)

func main() {
	context := &Context{}

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
	    LISTEN:       %s
	    PORT:         %d
	    NETWORK:      %s
	    RANGE_LOW:    %s
	    RANGE_HIGH:   %s
	    LOG_LEVEL:    %s
	    LOG_FORMAT:   %s`,
		config.Listen, config.Port,
		config.Network, config.RangeLow, config.RangeHigh,
		config.LogLevel, config.LogFormat)

	context.storage = &MemoryStorage{}
	context.storage.Init(config.Network, config.RangeLow, config.RangeHigh)

	router := mux.NewRouter()
	router.HandleFunc("/allocations/{mac}", context.ReleaseAllocationHandler).Methods("DELETE")
	router.HandleFunc("/allocations/{mac}", context.AllocationHandler).Methods("GET")
	router.HandleFunc("/allocations/", context.ListAllocationsHandler).Methods("GET")
	router.HandleFunc("/addresses/{ip}", context.FreeAddressHandler).Methods("DELETE")
	http.Handle("/", router)

	http.ListenAndServe(fmt.Sprintf("%s:%d", config.Listen, config.Port), nil)
}
