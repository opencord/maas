package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"net/http"
)

type Config struct {
	Port      int    `default:"4242"`
	Listen    string `default:"0.0.0.0"`
	Network   string `default:"10.0.0.0/24"`
	Skip      int    `default:"1"`
	LogLevel  string `default:"warning" envconfig:"LOG_LEVEL"`
	LogFormat string `default:"text" envconfig:"LOG_FORMAT"`
}

type Context struct {
	storage Storage
}

var log = logrus.New()

func main() {
	context := &Context{}

	config := Config{}
	err := envconfig.Process("ALLOCATE", &config)
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
	    Listen:       %s
	    Port:         %d
	    Network:      %s
	    SKip:         %d
	    Log Level:    %s
	    Log Format:   %s`, config.Listen, config.Port, config.Network, config.Skip,
		config.LogLevel, config.LogFormat)

	context.storage = &MemoryStorage{}
	context.storage.Init(config.Network, config.Skip)

	router := mux.NewRouter()
	router.HandleFunc("/allocations/{mac}", context.ReleaseAllocationHandler).Methods("DELETE")
	router.HandleFunc("/allocations/{mac}", context.AllocationHandler).Methods("GET")
	router.HandleFunc("/allocations/", context.ListAllocationsHandler).Methods("GET")
	router.HandleFunc("/addresses/{ip}", context.FreeAddressHandler).Methods("DELETE")
	http.Handle("/", router)

	http.ListenAndServe(fmt.Sprintf("%s:%d", config.Listen, config.Port), nil)
}
