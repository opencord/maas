package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"log"
	"net/http"
	"time"
)

type Config struct {
	VendorsURL      string `default:"file:///switchq/vendors.json" envconfig:"vendors_url"`
	StorageURL      string `default:"memory:" envconfig:"storage_url"`
	AddressURL      string `default:"file:///switchq/dhcp_harvest.inc" envconfig:"address_url"`
	PollInterval    string `default:"1m" envconfig:"poll_interval"`
	ProvisionTTL    string `default:"1h" envconfig:"provision_ttl"`
	ProvisionURL    string `default:"" envconfig:"provision_url"`
	RoleSelectorURL string `default:"" envconfig:"role_selector_url"`
	DefaultRole     string `default:"fabric-switch" envconfig:"default_role"`
	Script          string `default:"do-ansible"`

	vendors       Vendors
	storage       Storage
	addressSource AddressSource
	interval      time.Duration
	ttl           time.Duration
}

func checkError(err error, msg string, args ...interface{}) {
	if err != nil {
		log.Fatalf(msg, args...)
	}
}

func (c *Config) getProvisionedState(rec AddressRec) (int, string, error) {
	log.Printf("[debug] Fetching provisioned state of device '%s' (%s, %s)",
		rec.Name, rec.IP, rec.MAC)
	resp, err := http.Get(c.ProvisionURL + rec.MAC)
	if err != nil {
		log.Printf("[error] Error while retrieving provisioning state for device '%s (%s, %s)' : %s",
			rec.Name, rec.IP, rec.MAC, err)
		return -1, "", err
	}
	if resp.StatusCode != 404 && int(resp.StatusCode/100) != 2 {
		log.Printf("[error] Error while retrieving provisioning state for device '%s (%s, %s)' : %s",
			rec.Name, rec.IP, rec.MAC, resp.Status)
		return -1, "", fmt.Errorf(resp.Status)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		decoder := json.NewDecoder(resp.Body)
		var raw interface{}
		err = decoder.Decode(&raw)
		if err != nil {
			log.Printf("[error] Unmarshal provisioning service response for device '%s (%s, %s)' : %s",
				rec.Name, rec.IP, rec.MAC, err)
			return -1, "", err
		}
		status := raw.(map[string]interface{})
		switch int(status["status"].(float64)) {
		case 0, 1: // "PENDING", "RUNNING"
			return int(status["status"].(float64)), "", nil
		case 2: // "COMPLETE"
			return 2, "", nil
		case 3: // "FAILED"
			return 3, status["message"].(string), nil
		default:
			err = fmt.Errorf("unknown provisioning status : %d", status["status"])
			log.Printf("[error] received unknown provisioning status for device '%s (%s)' : %s",
				rec.Name, rec.MAC, err)
			return -1, "", err
		}
	}

	// If we end up here that means that no record was found in the provisioning, so return
	// a status of -1, w/o an error
	return -1, "", nil
}

func (c *Config) provision(rec AddressRec) error {
	log.Printf("[info] POSTing to '%s' for provisioning of '%s (%s)'", c.ProvisionURL, rec.Name, rec.MAC)
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
		log.Printf("[error] Unable to marshal provisioning data : %s", err)
		return err
	}
	req, err := http.NewRequest("POST", c.ProvisionURL, bytes.NewReader(b))
	if err != nil {
		log.Printf("[error] Unable to construct POST request to provisioner : %s", err)
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		log.Printf("[error] Unable to POST request to provisioner : %s", err)
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		log.Printf("[error] Provisioning request not accepted by provisioner : %s", resp.Status)
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
		log.Printf("[debug] host with IP '%s' and MAC '%s' and named '%s' not a known switch type",
			rec.IP, rec.MAC, rec.Name)
		return nil
	}

	last, err := c.storage.LastProvisioned(rec.MAC)
	if err != nil {
		return err
	}

	if last == nil {
		log.Printf("[debug] no TTL for device '%s' (%s, %s)",
			rec.Name, rec.IP, rec.MAC)
	} else {
		log.Printf("[debug] TTL for device '%s' (%s, %s) is %v",
			rec.Name, rec.IP, rec.MAC, *last)
	}

	// Verify if the provision status of the node is complete, if in an error state then TTL means
	// nothing
	state, message, err := c.getProvisionedState(rec)
	switch state {
	case 0, 1: // Pending or Running
		log.Printf("[debug] device '%s' (%s, %s) is being provisioned",
			rec.Name, rec.IP, rec.MAC)
		return nil
	case 2: // Complete
		log.Printf("[debug] device '%s' (%s, %s) has completed provisioning",
			rec.Name, rec.IP, rec.MAC)
		// If no last record then set the TTL
		if last == nil {
			now := time.Now()
			last = &now
			c.storage.MarkProvisioned(rec.MAC, last)
			log.Printf("[debug] Storing TTL for device '%s' (%s, %s) as %v",
				rec.Name, rec.IP, rec.MAC, now)
			return nil
		}
	case 3: // Failed
		log.Printf("[debug] device '%s' (%s, %s) failed last provisioning with message '%s', reattempt",
			rec.Name, rec.IP, rec.MAC, message)
		c.storage.ClearProvisioned(rec.MAC)
		last = nil
	default: // No record
	}

	// If TTL is 0 then we will only provision a switch once.
	if last == nil || (c.ttl > 0 && time.Since(*last) > c.ttl) {
		if last != nil {
			c.storage.ClearProvisioned(rec.MAC)
			log.Printf("[debug] device '%s' (%s, %s) TTL expired, reprovisioning",
				rec.Name, rec.IP, rec.MAC)
		}
		c.provision(rec)
	} else if c.ttl == 0 {
		log.Printf("[debug] device '%s' (%s, %s) has completed its one time provisioning, with a TTL set to %s",
			rec.Name, rec.IP, rec.MAC, c.ProvisionTTL)
	} else {
		log.Printf("[debug] device '%s' (%s, %s) has completed provisioning within the specified TTL of %s",
			rec.Name, rec.IP, rec.MAC, c.ProvisionTTL)
	}
	return nil
}

func main() {

	var err error
	config := Config{}
	envconfig.Process("SWITCHQ", &config)

	config.vendors, err = NewVendors(config.VendorsURL)
	checkError(err, "Unable to create known vendors list from specified URL '%s' : %s", config.VendorsURL, err)

	config.storage, err = NewStorage(config.StorageURL)
	checkError(err, "Unable to create require storage for specified URL '%s' : %s", config.StorageURL, err)

	config.addressSource, err = NewAddressSource(config.AddressURL)
	checkError(err, "Unable to create required address source for specified URL '%s' : %s", config.AddressURL, err)

	config.interval, err = time.ParseDuration(config.PollInterval)
	checkError(err, "Unable to parse specified poll interface '%s' : %s", config.PollInterval, err)

	config.ttl, err = time.ParseDuration(config.ProvisionTTL)
	checkError(err, "Unable to parse specified provision TTL value of '%s' : %s", config.ProvisionTTL, err)

	log.Printf(`Configuration:
		Vendors URL:       %s
		Storage URL:       %s
		Poll Interval:     %s
		Address Source:    %s
		Provision TTL:     %s
		Provision URL:     %s
		Role Selector URL: %s
		Default Role:      %s
		Script:            %s`,
		config.VendorsURL, config.StorageURL, config.PollInterval, config.AddressURL, config.ProvisionTTL,
		config.ProvisionURL, config.RoleSelectorURL, config.DefaultRole, config.Script)

	// We use two methods to attempt to find the MAC (hardware) address associated with an IP. The first
	// is to look in the table. The second is to send an ARP packet.
	for {
		log.Printf("[info] Checking for switches @ %s", time.Now())
		addresses, err := config.addressSource.GetAddresses()

		if err != nil {
			log.Printf("[error] unable to read addresses from address source : %s", err)
		} else {
			log.Printf("[info] Queried %d addresses from address source", len(addresses))

			for _, rec := range addresses {
				log.Printf("[debug] Processing %s(%s, %s)", rec.Name, rec.IP, rec.MAC)
				if err := config.processRecord(rec); err != nil {
					log.Printf("[error] Error when processing IP '%s' : %s", rec.IP, err)
				}
			}
		}

		time.Sleep(config.interval)
	}
}
