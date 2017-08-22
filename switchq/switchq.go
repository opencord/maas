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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	maas "github.com/juju/gomaasapi"
	"github.com/kelseyhightower/envconfig"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

const appName = "SWITCHQ"

type Config struct {
	VendorsURL      string `default:"file:///switchq/vendors.json" envconfig:"VENDORS_URL" desc:"URL that specifies supported vendor OUI information"`
	AddressURL      string `default:"file:///switchq/dhcp_harvest.inc" envconfig:"ADDRESS_URL" desc:"URL of service or file from which to query IP information"`
	PollInterval    string `default:"1m" envconfig:"POLL_INTERVAL" desc:"how often IP information should be queried and processed"`
	ProvisionTTL    string `default:"1h" envconfig:"PROVISION_TTL" desc:"duration to wait for a provisioning request before considering it failed"`
	ProvisionURL    string `default:"" envconfig:"PROVISION_URL" desc:"URL of provisioning service"`
	RoleSelectorURL string `default:"" envconfig:"ROLE_SELECTOR_URL" desc:"URL of service to query for switch role"`
	DefaultRole     string `default:"fabric-switch" envconfig:"DEFAULT_ROLE" desc:"default switch role"`
	Script          string `default:"do-ansible" desc:"script to run for provisioner"`
	LogLevel        string `default:"warning" envconfig:"LOG_LEVEL" desc:"detail level for logging"`
	LogFormat       string `default:"text" envconfig:"LOG_FORMAT" desc:"output format for logging, text or json"`
	Listen          string `default:"" desc:"IP on which to listen for requests"`
	Port            int    `default:"4244" desc:"port on which to listen for requests"`
	MaasURL         string `default:"http://localhost/MAAS" envconfig:"MAAS_URL" desc:"connection string for MAAS"`
	MaasKey         string `default:"" envconfig:"MAAS_API_KEY" desc:"API key for MAAS"`
	ShowApiKey      bool   `default:"false" envconfig:"MAAS_SHOW_API_KEY" desc:"display API key in log"`
	ApiKeyFile      string `default:"/secrets/maas_api_key" envconfig:"MAAS_API_KEY_FILE" desc:"file from which to read API key"`

	vendors       Vendors
	addressSource AddressSource
	interval      time.Duration
	ttl           time.Duration
}

const (
	Pending TaskStatus = iota
	Running
	Complete
	Failed
)

type RequestInfo struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	Ip           string `json:"ip"`
	Mac          string `json:"mac"`
	RoleSelector string `json:"role_selector"`
	Role         string `json:"role"`
	Script       string `json:"script"`
}

type TaskStatus uint8

type WorkRequest struct {
	Info   *RequestInfo
	Script string
	Role   string
}

type StatusMsg struct {
	Request   *WorkRequest `json:"request"`
	Worker    int          `json:"worker"`
	Status    TaskStatus   `json:"status"`
	Message   string       `json:"message"`
	Timestamp int64        `json:"timestamp"`
}

type AppContext struct {
	config Config

	maasClient  *maas.MAASObject
	pushChan    chan []AddressRec
	mutex       sync.RWMutex
	nextList    []AddressRec
	publishList []AddressRec
}

func checkError(err error, msg string, args ...interface{}) {
	if err != nil {
		log.Fatalf(msg, args...)
	}
}

func (c *AppContext) getProvisionedState(rec AddressRec) (*StatusMsg, error) {
	if len(c.config.ProvisionURL) == 0 {
		log.Warnf("Unable to fetch provisioning state of device '%s' (%s, %s) as no URL for the provisioner was specified",
			rec.Name, rec.IP, rec.MAC)
		return nil, fmt.Errorf("No URL for provisioner specified")
	}
	log.Debugf("Fetching provisioned state of device '%s' (%s, %s)",
		rec.Name, rec.IP, rec.MAC)
	resp, err := http.Get(c.config.ProvisionURL + rec.MAC)
	if err != nil {
		log.Errorf("Error while retrieving provisioning state for device '%s (%s, %s)' : %s",
			rec.Name, rec.IP, rec.MAC, err)
		return nil, err
	}
	if resp.StatusCode != 404 && int(resp.StatusCode/100) != 2 {
		log.Errorf("Error while retrieving provisioning state for device '%s (%s, %s)' : %s",
			rec.Name, rec.IP, rec.MAC, resp.Status)
		return nil, fmt.Errorf(resp.Status)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		decoder := json.NewDecoder(resp.Body)
		var status StatusMsg
		err = decoder.Decode(&status)
		if err != nil {
			log.Errorf("Unmarshal provisioning service response for device '%s (%s, %s)' : %s",
				rec.Name, rec.IP, rec.MAC, err)
			return nil, err
		}
		return &status, nil
	}

	// If we end up here that means that no record was found in the provisioning, so return
	// a status of -1, w/o an error
	return nil, nil
}

func (c *AppContext) provision(rec AddressRec) error {
	if len(c.config.ProvisionURL) == 0 {
		log.Warnf("Unable to POST to provisioner for device '%s' (%s, %s) as no URL for the provisioner was specified",
			rec.Name, rec.IP, rec.MAC)
		return fmt.Errorf("No URL for provisioner specified")
	}
	log.Infof("POSTing to '%s' for provisioning of '%s (%s)'", c.config.ProvisionURL, rec.Name, rec.MAC)
	data := map[string]string{
		"id":   rec.MAC,
		"name": rec.Name,
		"ip":   rec.IP,
		"mac":  rec.MAC,
	}
	if c.config.RoleSelectorURL != "" {
		data["role_selector"] = c.config.RoleSelectorURL
	}
	if c.config.DefaultRole != "" {
		data["role"] = c.config.DefaultRole
	}
	if c.config.Script != "" {
		data["script"] = c.config.Script
	}

	hc := http.Client{}
	var b []byte
	b, err := json.Marshal(data)
	if err != nil {
		log.Errorf("Unable to marshal provisioning data : %s", err)
		return err
	}
	req, err := http.NewRequest("POST", c.config.ProvisionURL, bytes.NewReader(b))
	if err != nil {
		log.Errorf("Unable to construct POST request to provisioner : %s", err)
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		log.Errorf("Unable to POST request to provisioner : %s", err)
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		log.Errorf("Provisioning request not accepted by provisioner : %s", resp.Status)
		return err
	}

	return nil
}

func (c *AppContext) processRecord(rec AddressRec) error {
	ok, err := c.config.vendors.Switchq(rec.MAC)
	if err != nil {
		return fmt.Errorf("unable to determine ventor of MAC '%s' (%s)", rec.MAC, err)
	}

	if !ok {
		// Not something we care about
		log.Debugf("host with IP '%s' and MAC '%s' and named '%s' not a known switch type",
			rec.IP, rec.MAC, rec.Name)
		return nil
	}

	// Add this IP information to our list of known switches
	c.nextList = append(c.nextList, rec)

	// Verify if the provision status of the node is complete, if in an error state then TTL means
	// nothing
	state, err := c.getProvisionedState(rec)
	if state != nil {
		switch state.Status {
		case Pending, Running: // Pending or Running
			log.Debugf("device '%s' (%s, %s) is being provisioned",
				rec.Name, rec.IP, rec.MAC)
			return nil
		case Complete: // Complete
			log.Debugf("device '%s' (%s, %s) has completed provisioning",
				rec.Name, rec.IP, rec.MAC)
		case Failed: // Failed
			log.Debugf("device '%s' (%s, %s) failed last provisioning with message '%s', reattempt",
				rec.Name, rec.IP, rec.MAC, state.Message)
			state = nil
		default: // Unknown state
			log.Debugf("device '%s' (%s, %s) has unknown provisioning state '%d', will provision",
				rec.Name, rec.IP, rec.MAC, state.Status)
			state = nil
		}
	} else {
		log.Debugf("device '%s' (%s, %s) has no provisioning record",
			rec.Name, rec.IP, rec.MAC)
	}

	// If TTL is 0 then we will only provision a switch once.
	if state == nil || (c.config.ttl > 0 && time.Since(time.Unix(state.Timestamp, 0)) > c.config.ttl) {
		if state != nil {
			log.Debugf("device '%s' (%s, %s) TTL expired, reprovisioning",
				rec.Name, rec.IP, rec.MAC)
		}
		c.provision(rec)
	} else if c.config.ttl == 0 {
		log.Debugf("device '%s' (%s, %s) has completed its one time provisioning, with a TTL set to %s",
			rec.Name, rec.IP, rec.MAC, c.config.ProvisionTTL)
	} else {
		log.Debugf("device '%s' (%s, %s) has completed provisioning within the specified TTL of %s",
			rec.Name, rec.IP, rec.MAC, c.config.ProvisionTTL)
	}
	return nil
}

func (c *AppContext) processLoop() {
	// We use two methods to attempt to find the MAC (hardware) address associated with an IP. The first
	// is to look in the table. The second is to send an ARP packet.
	for {
		log.Infof("Checking for switches @ %s", time.Now())
		addresses, err := c.config.addressSource.GetAddresses()

		if err != nil {
			log.Errorf("unable to read addresses from address source : %s", err)
		} else {
			log.Infof("Queried %d addresses from address source", len(addresses))

			c.nextList = make([]AddressRec, 0, len(addresses))
			for _, rec := range addresses {
				log.Debugf("Processing %s(%s, %s)", rec.Name, rec.IP, rec.MAC)
				if err := c.processRecord(rec); err != nil {
					log.Errorf("Error when processing IP '%s' : %s", rec.IP, err)
				}
			}
			c.mutex.Lock()
			c.publishList = c.nextList
			c.nextList = nil
			c.mutex.Unlock()
			c.pushChan <- c.publishList
		}

		time.Sleep(c.config.interval)
	}
}

var log = logrus.New()
var appFlags = flag.NewFlagSet("", flag.ContinueOnError)

func main() {

	var err error
	context := &AppContext{}

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
	err = envconfig.Process(appName, &context.config)
	if err != nil {
		log.Fatalf("Unable to parse configuration options : %s", err)
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

	re := regexp.MustCompile("[^:]")
	pubKey := context.config.MaasKey
	if !context.config.ShowApiKey {
		pubKey = re.ReplaceAllString(context.config.MaasKey, "X")
	}

	log.Infof(`Configuration:
		VENDORS_URL:       %s
		POLL_INTERVAL:     %s
		ADDRESS_URL:       %s
		PROVISION_TTL:     %s
		PROVISION_URL:     %s
		ROLE_SELECTOR_URL: %s
		DEFAULT_ROLE:      %s
		SCRIPT:            %s
		LISTEN:            %s
		PORT:              %d
		MAAS_URL:          %s
		MAAS_SHOW_API_KEY  %t
		MAAS_API_KEY:      %s
		MAAS_API_KEY_FILE: %s
		LOG_LEVEL:         %s
		LOG_FORMAT:        %s`,
		context.config.VendorsURL, context.config.PollInterval, context.config.AddressURL, context.config.ProvisionTTL,
		context.config.ProvisionURL, context.config.RoleSelectorURL, context.config.DefaultRole, context.config.Script,
		context.config.Listen, context.config.Port, context.config.MaasURL, context.config.ShowApiKey, pubKey,
		context.config.ApiKeyFile, context.config.LogLevel, context.config.LogFormat)

	context.config.vendors, err = NewVendors(context.config.VendorsURL)
	checkError(err, "Unable to create known vendors list from specified URL '%s' : %s", context.config.VendorsURL, err)

	context.config.addressSource, err = NewAddressSource(context.config.AddressURL)
	checkError(err, "Unable to create required address source for specified URL '%s' : %s", context.config.AddressURL, err)

	context.config.interval, err = time.ParseDuration(context.config.PollInterval)
	checkError(err, "Unable to parse specified poll interface '%s' : %s", context.config.PollInterval, err)

	context.config.ttl, err = time.ParseDuration(context.config.ProvisionTTL)
	checkError(err, "Unable to parse specified provision TTL value of '%s' : %s", context.config.ProvisionTTL, err)

	// Attempt to load the API key from a file if it was not set via the environment
	// and if the file exists
	if context.config.MaasKey == "" {
		log.Debugf("Attempting to read MAAS API key from file '%s', because it was not set via environment", context.config.ApiKeyFile)
		keyBytes, err := ioutil.ReadFile(context.config.ApiKeyFile)
		if err != nil {
			log.Warnf("Failed to read MAAS API key from file '%s', was the file mounted as a volume? : %s ",
				context.config.ApiKeyFile, err)
		} else {
			context.config.MaasKey = string(keyBytes)
			if context.config.ShowApiKey {
				pubKey = context.config.MaasKey
			} else {
				pubKey = re.ReplaceAllString(context.config.MaasKey, "X")
			}
		}
	}

	if len(context.config.MaasURL) > 0 {

		// Attempt to connect to MAAS
		authClient, err := maas.NewAuthenticatedClient(context.config.MaasURL, context.config.MaasKey, "1.0")
		checkError(err, "Unable to connect to MAAS at '%s' : %s", context.config.MaasURL, err)

		context.maasClient = maas.NewMAAS(*authClient)
	}

	context.pushChan = make(chan []AddressRec, 1)

	go context.processLoop()
	go context.syncToMaas(context.pushChan)

	router := mux.NewRouter()
	router.HandleFunc("/switch/", context.ListSwitchesHandler).Methods("GET")
	http.Handle("/", router)
	log.Infof("Listening for HTTP request on '%s:%d'", context.config.Listen, context.config.Port)
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", context.config.Listen, context.config.Port), nil)
	if err != nil {
		checkError(err, "Error while attempting to listen to REST requests on '%s:%d' : %s",
			context.config.Listen, context.config.Port, err)
	}
}
