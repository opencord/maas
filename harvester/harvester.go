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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

const appName = "HARVESTER"

// application application configuration and internal state
type application struct {
	Port                int           `default:"4246" desc:"port on which the service will listen for requests"`
	Listen              string        `default:"0.0.0.0" desc:"IP on which the service will listen for requests"`
	LogLevel            string        `default:"warning" envconfig:"LOG_LEVEL" desc:"log output level"`
	LogFormat           string        `default:"text" envconfig:"LOG_FORMAT" desc:"format of log messages"`
	DHCPLeaseFile       string        `default:"/harvester/dhcpd.leases" envconfig:"DHCP_LEASE_FILE" desc:"lease file to parse for lease information"`
	DHCPReservationFile string        `default:"/reservations/dhcpd.reservations" envconfig:"DHCP_RESERVATION_FILE" desc:"lease reservation file for IP information"`
	OutputFile          string        `envconfig:"OUTPUT_FILE" desc:"name of file to output discovered lease in bind9 format"`
	OutputFormat        string        `default:"{{.ClientHostname}}\tIN A {{.IPAddress}}\t; {{.HardwareAddress}}" envconfig:"OUTPUT_FORMAT" desc:"specifies the single entry format when outputing to a file"`
	VerifyLeases        bool          `default:"true" envconfig:"VERIFY_LEASES" desc:"verifies leases with a ping"`
	VerifyTimeout       time.Duration `default:"1s" envconfig:"VERIFY_TIMEOUT" desc:"max timeout (RTT) to wait for verification pings"`
	VerifyWithUDP       bool          `default:"false" envconfig:"VERIFY_WITH_UDP" desc:"use UDP instead of raw sockets for ping verification"`
	QueryPeriod         time.Duration `default:"30s" envconfig:"QUERY_PERIOD" desc:"period at which the DHCP lease file is processed"`
	QuietPeriod         time.Duration `default:"2s" envconfing:"QUIET_PERIOD" desc:"period to wait between accepting parse requests"`
	RequestTimeout      time.Duration `default:"10s" envconfig:"REQUEST_TIMEOUT" desc:"period to wait for processing when requesting a DHCP lease database parsing"`
	RNDCUpdate          bool          `default:"false" envconfig:"RNDC_UPDATE" desc:"determines if the harvester reloads the DNS servers after harvest"`
	RNDCAddress         string        `default:"127.0.0.1" envconfig:"RNDC_ADDRESS" desc:"IP address of the DNS server to contact via RNDC"`
	RNDCPort            int           `default:"954" envconfig:"RNDC_PORT" desc:"port of the DNS server to contact via RNDC"`
	RNDCKeyFile         string        `default:"/key/rndc.conf.maas" envconfig:"RNDC_KEY_FILE" desc:"key file, with default, to contact DNS server"`
	RNDCZone            string        `default:"cord.lab" envconfig:"RNDC_ZONE" desc:"zone to reload"`
	BadClientNames      []string      `default:"localhost" envconfig:"BAD_CLIENT_NAMES" desc:"list of invalid hostnames for clients"`
	ClientNameTemplate  string        `default:"UKN-{{with $x:=.HardwareAddress|print}}{{regex $x \":\" \"\"}}{{end}}" envconfig:"CLIENT_NAME_TEMPLATE" desc:"template for generated host name"`

	appFlags           *flag.FlagSet      `ignored:"true"`
	log                *logrus.Logger     `ignored:"true"`
	interchange        sync.RWMutex       `ignored:"true"`
	leases             map[string]*Lease  `ignored:"true"`
	byHardware         map[string]*Lease  `ignored:"true"`
	byHostname         map[string]*Lease  `ignored:"true"`
	outputTemplate     *template.Template `ignored:"true"`
	requests           chan *chan uint    `ignored:"true"`
	clientNameTemplate *template.Template `ignored:"true"`
	badClientNames     map[string]bool    `ignored:"true"`
}

func main() {

	// initialize application state
	app := &application{
		log:      logrus.New(),
		appFlags: flag.NewFlagSet("", flag.ContinueOnError),
		requests: make(chan *chan uint, 100),
	}

	app.appFlags.Usage = func() {
		envconfig.Usage(appName, app)
	}
	if err := app.appFlags.Parse(os.Args[1:]); err != nil {
		if err != flag.ErrHelp {
			os.Exit(1)
		} else {
			return
		}
	}

	// process and validate the application configuration
	err := envconfig.Process("HARVESTER", app)
	if err != nil {
		app.log.Fatalf("unable to parse configuration options : %s", err)
	}
	switch app.LogFormat {
	case "json":
		app.log.Formatter = &logrus.JSONFormatter{}
	default:
		app.log.Formatter = &logrus.TextFormatter{
			FullTimestamp: true,
			ForceColors:   true,
		}
	}
	level, err := logrus.ParseLevel(app.LogLevel)
	if err != nil {
		level = logrus.WarnLevel
	}
	app.log.Level = level

	app.outputTemplate, err = template.New("harvester").Parse(app.OutputFormat)
	if err != nil {
		app.log.Fatalf("invalid output file format specified : %s", err)
	}

	// output the configuration
	app.log.Infof(`Configuration:
           LISTEN:                %s
           PORT:                  %d
           LOG_LEVEL:             %s
           LOG_FORMAT:            %s
           DHCP_LEASE_FILE:       %s
           DHCP_RESERVATION_FILE: %s
           OUTPUT_FILE:           %s
           OUTPUT_FORMAT:         %s
           VERIFY_LEASES:         %t
           VERIFY_TIMEOUT:        %s
           VERIFY_WITH_UDP:       %t
           QUERY_PERIOD:          %s
           QUIET_PERIOD:          %s
           REQUEST_TIMEOUT:       %s
           RNDC_UPDATE:           %t
           RNDC_ADDRESS:          %s
           RNDC_PORT:             %d
           RNDC_KEY_FILE:         %s
           RNDC_ZONE:             %s
	   BAD_CLIENT_NAMES:      %s
	   CLIENT_NAME_TEMPLATE:  %s`,
		app.Listen, app.Port,
		app.LogLevel, app.LogFormat,
		app.DHCPLeaseFile, app.DHCPReservationFile, app.OutputFile, strconv.Quote(app.OutputFormat),
		app.VerifyLeases, app.VerifyTimeout, app.VerifyWithUDP,
		app.QueryPeriod, app.QuietPeriod, app.RequestTimeout,
		app.RNDCUpdate, app.RNDCAddress, app.RNDCPort, app.RNDCKeyFile, app.RNDCZone,
		strings.Join(app.BadClientNames[:], ","), app.ClientNameTemplate)

	app.clientNameTemplate, err = template.New("harvester").Funcs(template.FuncMap{
		"regex": func(target, match, replace string) string {
			re := regexp.MustCompile(match)
			return re.ReplaceAllString(target, replace)
		},
	}).Parse(app.ClientNameTemplate)
	if err != nil {
		app.log.Fatalf("Unable to parse client host name template %s", err)
	}

	app.badClientNames = make(map[string]bool)
	for _, bad := range app.BadClientNames {
		app.badClientNames[bad] = true
	}

	// establish REST end points
	router := mux.NewRouter()
	router.HandleFunc("/lease/", app.listLeasesHandler).Methods("GET")
	router.HandleFunc("/lease/{ip}", app.getLeaseHandler).Methods("GET")
	router.HandleFunc("/lease/hardware/{mac}", app.getLeaseByHardware).Methods("GET")
	router.HandleFunc("/lease/hostname/{name}", app.getLeaseByHostname).Methods("GET")
	router.HandleFunc("/harvest/", app.doHarvestHandler).Methods("POST")
	router.HandleFunc("/harvest", app.doHarvestHandler).Methods("POST")
	http.Handle("/", router)

	// start DHCP lease file synchronization handler
	go app.syncRequestHandler(app.requests)

	// start loop to periodically synchronize DHCP lease file
	go app.syncFromDHCPLeaseFileLoop(app.requests)

	// listen for REST requests
	http.ListenAndServe(fmt.Sprintf("%s:%d", app.Listen, app.Port), nil)
}
