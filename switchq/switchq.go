package main

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"log"
	"time"
)

type Config struct {
	StorageURL   string `default:"memory:///switchq/vendors.json" envconfig:"storage_url"`
	AddressURL   string `default:"file:///switchq/dhcp_harvest.inc" envconfig:"address_url"`
	PollInterval string `default:"1m" envconfig:"poll_interval"`
	ProvisionTTL string `default:"1h" envconfig:"check_ttl"`

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

func (c *Config) processRecord(rec AddressRec) error {
	if c.ttl == 0 {
		// One provisioning only please
		return nil
	}

	ok, err := c.storage.Switchq(rec.MAC)
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
	if last == nil || time.Since(*last) > c.ttl {
		log.Printf("[debug] time to provision %s", rec.MAC)
	}
	return nil
}

func main() {

	var err error
	config := Config{}
	envconfig.Process("SWITCHQ", &config)

	config.storage, err = NewStorage(config.StorageURL)
	checkError(err, "Unable to create require storage for specified URL '%s' : %s", config.StorageURL, err)

	config.addressSource, err = NewAddressSource(config.AddressURL)
	checkError(err, "Unable to create required address source for specified URL '%s' : %s", config.AddressURL, err)

	config.interval, err = time.ParseDuration(config.PollInterval)
	checkError(err, "Unable to parse specified poll interface '%s' : %s", config.PollInterval, err)

	config.ttl, err = time.ParseDuration(config.ProvisionTTL)
	checkError(err, "Unable to parse specified provision TTL value of '%s' : %s", config.ProvisionTTL, err)

	log.Printf(`Configuration:
		Storage URL:    %s
		Poll Interval:  %s
		Address Source: %s
		Provision TTL:  %s`,
		config.StorageURL, config.PollInterval, config.AddressURL, config.ProvisionTTL)

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
