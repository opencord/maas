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
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/kelseyhightower/envconfig"
	"net/http"
	"time"
)

type Config struct {
	VendorsURL      string `default:"file:///switchq/vendors.json" envconfig:"vendors_url"`
	AddressURL      string `default:"file:///switchq/dhcp_harvest.inc" envconfig:"address_url"`
	PollInterval    string `default:"1m" envconfig:"poll_interval"`
	ProvisionTTL    string `default:"1h" envconfig:"provision_ttl"`
	ProvisionURL    string `default:"" envconfig:"provision_url"`
	RoleSelectorURL string `default:"" envconfig:"role_selector_url"`
	DefaultRole     string `default:"fabric-switch" envconfig:"default_role"`
	Script          string `default:"do-ansible"`
	LogLevel        string `default:"warning" envconfig:"LOG_LEVEL"`
	LogFormat       string `default:"text" envconfig:"LOG_FORMAT"`

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

func checkError(err error, msg string, args ...interface{}) {
	if err != nil {
		log.Fatalf(msg, args...)
	}
}

func (c *Config) getProvisionedState(rec AddressRec) (*StatusMsg, error) {
	log.Debugf("Fetching provisioned state of device '%s' (%s, %s)",
		rec.Name, rec.IP, rec.MAC)
	resp, err := http.Get(c.ProvisionURL + rec.MAC)
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

func (c *Config) provision(rec AddressRec) error {
	log.Infof("POSTing to '%s' for provisioning of '%s (%s)'", c.ProvisionURL, rec.Name, rec.MAC)
	data := map[string]string{
		"id":   rec.MAC,
		"name": rec.Name,
		"ip":   rec.IP,
		"mac":  rec.MAC,
	}
	if c.RoleSelectorURL != "" {
		data["role_selector"] = c.RoleSelectorURL
	}
	if c.DefaultRole != "" {
		data["role"] = c.DefaultRole
	}
	if c.Script != "" {
		data["script"] = c.Script
	}

	hc := http.Client{}
	var b []byte
	b, err := json.Marshal(data)
	if err != nil {
		log.Errorf("Unable to marshal provisioning data : %s", err)
		return err
	}
	req, err := http.NewRequest("POST", c.ProvisionURL, bytes.NewReader(b))
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

func (c *Config) processRecord(rec AddressRec) error {
	ok, err := c.vendors.Switchq(rec.MAC)
	if err != nil {
		return fmt.Errorf("unable to determine ventor of MAC '%s' (%s)", rec.MAC, err)
	}

	if !ok {
		// Not something we care about
		log.Debugf("host with IP '%s' and MAC '%s' and named '%s' not a known switch type",
			rec.IP, rec.MAC, rec.Name)
		return nil
	}

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
		default: // Unknown state
			log.Debugf("device '%s' (%s, %s) has unknown provisioning state '%d', will provision",
				rec.Name, rec.IP, rec.MAC, state.Status)
		}
	} else {
		log.Debugf("device '%s' (%s, %s) has no provisioning record",
			rec.Name, rec.IP, rec.MAC)
	}

	// If TTL is 0 then we will only provision a switch once.
	if state == nil || (c.ttl > 0 && time.Since(time.Unix(state.Timestamp, 0)) > c.ttl) {
		if state != nil {
			log.Debugf("device '%s' (%s, %s) TTL expired, reprovisioning",
				rec.Name, rec.IP, rec.MAC)
		}
		c.provision(rec)
	} else if c.ttl == 0 {
		log.Debugf("device '%s' (%s, %s) has completed its one time provisioning, with a TTL set to %s",
			rec.Name, rec.IP, rec.MAC, c.ProvisionTTL)
	} else {
		log.Debugf("device '%s' (%s, %s) has completed provisioning within the specified TTL of %s",
			rec.Name, rec.IP, rec.MAC, c.ProvisionTTL)
	}
	return nil
}

var log = logrus.New()

func main() {

	var err error
	config := Config{}
	err = envconfig.Process("SWITCHQ", &config)
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

	config.vendors, err = NewVendors(config.VendorsURL)
	checkError(err, "Unable to create known vendors list from specified URL '%s' : %s", config.VendorsURL, err)

	config.addressSource, err = NewAddressSource(config.AddressURL)
	checkError(err, "Unable to create required address source for specified URL '%s' : %s", config.AddressURL, err)

	config.interval, err = time.ParseDuration(config.PollInterval)
	checkError(err, "Unable to parse specified poll interface '%s' : %s", config.PollInterval, err)

	config.ttl, err = time.ParseDuration(config.ProvisionTTL)
	checkError(err, "Unable to parse specified provision TTL value of '%s' : %s", config.ProvisionTTL, err)

	log.Infof(`Configuration:
		Vendors URL:       %s
		Poll Interval:     %s
		Address Source:    %s
		Provision TTL:     %s
		Provision URL:     %s
		Role Selector URL: %s
		Default Role:      %s
		Script:            %s
		Log Level:         %s
		Log Format:        %s`,
		config.VendorsURL, config.PollInterval, config.AddressURL, config.ProvisionTTL,
		config.ProvisionURL, config.RoleSelectorURL, config.DefaultRole, config.Script,
		config.LogLevel, config.LogFormat)

	// We use two methods to attempt to find the MAC (hardware) address associated with an IP. The first
	// is to look in the table. The second is to send an ARP packet.
	for {
		log.Infof("Checking for switches @ %s", time.Now())
		addresses, err := config.addressSource.GetAddresses()

		if err != nil {
			log.Errorf("unable to read addresses from address source : %s", err)
		} else {
			log.Infof("Queried %d addresses from address source", len(addresses))

			for _, rec := range addresses {
				log.Debugf("Processing %s(%s, %s)", rec.Name, rec.IP, rec.MAC)
				if err := config.processRecord(rec); err != nil {
					log.Errorf("Error when processing IP '%s' : %s", rec.IP, err)
				}
			}
		}

		time.Sleep(config.interval)
	}
}
