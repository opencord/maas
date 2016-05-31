package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"log"
	"net/http"
)

type Config struct {
	Port         int    `default:"4242"`
	Listen       string `default:"0.0.0.0"`
	StartAddress string `default:"10.0.0.2" envconfig:"start_address"`
	AddressCount uint   `default:"252" envconfig:"address_count"`
}

type Context struct {
	storage Storage
}

func main() {
	context := &Context{}

	config := Config{}
	err := envconfig.Process("ALLOCATE", &config)
	if err != nil {
		log.Fatalf("[error] Unable to parse configuration options : %s", err)
	}

	log.Printf(`Configuration:
	    Listen:       %s
	    Port:         %d
	    StartAddress: %s
	    AddressCount: %d`, config.Listen, config.Port, config.StartAddress, config.AddressCount)

	context.storage = &MemoryStorage{}
	context.storage.Init(config.StartAddress, config.AddressCount)

	router := mux.NewRouter()
	router.HandleFunc("/allocations/{mac}", context.ReleaseAllocationHandler).Methods("DELETE")
	router.HandleFunc("/allocations/{mac}", context.AllocationHandler).Methods("GET")
	router.HandleFunc("/allocations/", context.ListAllocationsHandler).Methods("GET")
	router.HandleFunc("/addresses/{ip}", context.FreeAddressHandler).Methods("DELETE")
	http.Handle("/", router)

	http.ListenAndServe(fmt.Sprintf("%s:%d", config.Listen, config.Port), nil)
}
