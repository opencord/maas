package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"log"
	"net/http"
)

type Config struct {
	Port            int    `default:"4243"`
	Listen          string `default:"0.0.0.0"`
	RoleSelectorURL string `default:"" envconfig:"role_selector_url"`
	DefaultRole     string `default:"compute-node" envconfig:"default_role"`
	Script          string `default:"do-ansible"`
}

type Context struct {
	config     Config
	storage    Storage
	workers    []Worker
	dispatcher *Dispatcher
}

func main() {
	context := &Context{}

	err := envconfig.Process("PROVISION", &(context.config))
	if err != nil {
		log.Fatalf("[error] Unable to parse configuration options : %s", err)
	}

	log.Printf(`Configuration:
	    Listen:          %s
	    Port:            %d
	    RoleSelectorURL: %s
	    DefaultRole:     %s`,
		context.config.Listen, context.config.Port, context.config.RoleSelectorURL, context.config.DefaultRole)

	context.storage = NewMemoryStorage()

	router := mux.NewRouter()
	router.HandleFunc("/provision/", context.ProvisionRequestHandler).Methods("POST")
	router.HandleFunc("/provision/", context.ListRequestsHandler).Methods("GET")
	router.HandleFunc("/provision/{nodeid}", context.QueryStatusHandler).Methods("GET")
	http.Handle("/", router)

	// Start the dispatcher and workers
	context.dispatcher = NewDispatcher(5, context.storage)
	context.dispatcher.Start()

	http.ListenAndServe(fmt.Sprintf("%s:%d", context.config.Listen, context.config.Port), nil)
}
