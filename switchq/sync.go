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
	"fmt"
	maas "github.com/juju/gomaasapi"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type MatchRec struct {
	AddressRec
	ResourceUri string
}

type subnetRec struct {
	name   string
	cidr   *net.IPNet
	vlanID string
}

func getSubnetForAddr(subnets []subnetRec, ip string) *subnetRec {
	asIp := net.ParseIP(ip)
	for _, rec := range subnets {
		if rec.cidr.Contains(asIp) {
			return &rec
		}
	}
	return nil
}

// process converts the device list from MAAS into a maps to quickly lookup a record by name
func process(deviceList []maas.JSONObject) (byName map[string]*MatchRec, byMac map[string]*MatchRec, err error) {
	byName = make(map[string]*MatchRec)
	byMac = make(map[string]*MatchRec)
	all := make([]MatchRec, len(deviceList))

	for i, deviceObj := range deviceList {
		device, err := deviceObj.GetMap()
		if err != nil {
			return nil, nil, err
		}

		uri, err := device["resource_uri"].GetString()
		if err != nil {
			return nil, nil, err
		}
		all[i].ResourceUri = uri

		name, err := device["hostname"].GetString()
		if err != nil {
			return nil, nil, err
		}

		// Strip the domain from the hostname
		idx := strings.Index(name, ".")
		if idx != -1 {
			name = name[:idx]
		}
		all[i].Name = name

		mac_set_arr, err := device["macaddress_set"].GetArray()
		if len(mac_set_arr) != 1 {
			return nil, nil, fmt.Errorf("Expecting a single MAC address, recived %d", len(mac_set_arr))
		}

		mac_obj, err := mac_set_arr[0].GetMap()
		if err != nil {
			return nil, nil, err
		}

		mac, err := mac_obj["mac_address"].GetString()
		if err != nil {
			return nil, nil, err
		}
		mac = strings.ToUpper(mac)
		all[i].MAC = mac

		byName[name] = &all[i]
		byMac[mac] = &all[i]
	}

	return
}

// synctoMaas checks to see if the devices is already in MAAS and if not adds it containment is determined by a matching
// hostname and MAC address. if there is not match then a new devie is POSTed to MAAS
func (c *AppContext) syncToMaas(request chan []AddressRec) {
	log.Info("Starting MAAS Switch Synchronizer")

	// Wait for request
	for list := range request {
		// Get current device list and convert it to some maps for quick indexing
		devices := c.maasClient.GetSubObject("devices")
		deviceObjects, err := devices.CallGet("list", url.Values{})
		if err != nil {
			log.Errorf("Unable to synchronize switches to MAAS, unable to get current devices : %s",
				err)
			break
		}
		deviceList, err := deviceObjects.GetArray()
		if err != nil {
			log.Errorf("Unable to synchronize switches to MAAS, unable to deserialize devices : %s",
				err)
			break
		}
		byName, byMac, err := process(deviceList)
		if err != nil {
			log.Errorf("Unable to process current device list : %s", err)
			return
		}

		// Get all the subnets from MAAS and store them in a local array for quick access. The subnets
		// are used to attempt to map the switch into a subnet based on its IP address
		subnets := c.maasClient.GetSubObject("subnets")
		subnetObjects, err := subnets.CallGet("", url.Values{})
		if err != nil {
			log.Errorf("Unable to retrieve subnets from MAAS : %s", err)
			return
		}

		subnetArr, err := subnetObjects.GetArray()
		if err != nil {
			log.Errorf("Unable to get subnet array from MAAS : %s", err)
			return
		}

		subnetRecs := make([]subnetRec, len(subnetArr))
		for i, subnetObj := range subnetArr {
			subnet, err := subnetObj.GetMap()
			if err != nil {
				log.Errorf("Unable to process subnet from MAAS : %s", err)
				return
			}

			name, err := subnet["name"].GetString()
			if err != nil {
				log.Errorf("Unable to get Name from MAAS subnet : %s", err)
				return
			}

			s_cidr, err := subnet["cidr"].GetString()
			if err != nil {
				log.Errorf("Unable to get CIDR from MAAS subnet : %s", err)
				return
			}
			_, cidr, err := net.ParseCIDR(s_cidr)
			if err != nil {
				log.Errorf("Unable to parse CIDR '%s' from MAAS : %s", s_cidr, err)
				return
			}

			vlanMap, err := subnet["vlan"].GetMap()
			if err != nil {
				log.Errorf("Unable to get vlan for MAAS subnet '%s' : %s", s_cidr, err)
				return
			}

			id, err := vlanMap["id"].GetFloat64()
			if err != nil {
				log.Errorf("Unable to get VLAN ID for MAAS subnet '%s' : %s", s_cidr, err)
				return
			}
			subnetRecs[i].name = name
			subnetRecs[i].cidr = cidr
			subnetRecs[i].vlanID = strconv.Itoa(int(id))
		}

		// Iterage over the list of devices to sync to MAAS
		for _, rec := range list {
			// First check for matching hostname
			found, ok := byName[rec.Name]
			if ok {
				// Found an existing record with a matching hostname. If the MAC matches then
				// this means this device is already in MAAS and we are good.
				if strings.ToUpper(rec.MAC) == found.MAC {
					// All is good
					log.Infof("Device '%s (%s)' already in MAAS", rec.Name, rec.MAC)
					continue
				} else {
					// Have a matching hostname, but a different MAC address. Can't
					// push a duplicate hostname to MAAS. As the MAC is considered the
					// unique identifier we will append the MAC address to the given
					// hostname and add the device under that host name
					log.Warnf("Device '%s (%s)' exists in MAAS with a different MAC, augmenting hostname with MAC to form unique hostname",
						rec.Name, rec.MAC)
					namePlus := rec.Name + "-" + strings.Replace(strings.ToLower(rec.MAC), ":", "", -1)
					_, ok = byName[namePlus]
					if ok {
						// A record with the name + mac already exists, assume this is the
						// same record then and all is well
						log.Infof("Device '%s (%s)' already in MAAS", namePlus, rec.MAC)
						continue
					}

					// Modify the record so that it will be created with the new name
					rec.Name = namePlus
				}
			}
			found, ok = byMac[strings.ToUpper(rec.MAC)]
			if ok {
				// Found a record with this MAC address, but a different hostname. Update
				// the hostname to the correct value
				log.Infof("Device with matching MAC, but different name found, updating name to '%s (%s)'",
					rec.Name, rec.MAC)
				deviceObj := c.maasClient.GetSubObject(found.ResourceUri)
				_, err := deviceObj.Update(url.Values{
					"hostname": []string{rec.Name},
				})
				if err != nil {
					log.Errorf("Unable to update hostname for device '%s (%s)' in MAAS : %s",
						rec.Name, rec.MAC, err)
				}
				continue
			}

			// The device does not currently exist in MAAS, so add it
			log.Infof("Adding device '%s (%s)' to MAAS", rec.Name, rec.MAC)
			deviceObj, err := devices.CallPost("new", url.Values{
				"hostname":      []string{rec.Name},
				"mac_addresses": []string{rec.MAC},
			})
			if err != nil {
				log.Errorf("Unable to synchronize switch '%s' (%s, %s) to MAAS : %s",
					rec.Name, rec.IP, rec.MAC, err)
				continue
			}

			// Get the interface of the device so if can be assigned to a subnet
			deviceMap, err := deviceObj.GetMap()
			if err != nil {
				log.Errorf("Can't get device object for '%s (%s)' : %s",
					rec.Name, rec.MAC, err)
				continue
			}

			interfaceSetArr, err := deviceMap["interface_set"].GetArray()
			if err != nil {
				log.Errorf("Can't get device interface set for '%s (%s)' : %s",
					rec.Name, rec.MAC, err)
				continue
			}

			ifaceMap, err := interfaceSetArr[0].GetMap()
			if err != nil {
				log.Errorf("Unable to get first interface for '%s (%s)' : %s",
					rec.Name, rec.MAC, err)
				continue
			}

			ifaceUri, err := ifaceMap["resource_uri"].GetString()
			if err != nil {
				log.Errorf("Unable to get interface URI for '%s (%s)' : %s",
					rec.Name, rec.MAC, err)
				continue
			}

			// Get the appropriate subnect for the switches IP. If one cannot be found then
			// nothing can be done
			subnetRec := getSubnetForAddr(subnetRecs, rec.IP)
			if subnetRec == nil {
				log.Errorf("Unable to find VLAN ID for '%s (%s)' using IP '%s'",
					rec.Name, rec.MAC, rec.IP)
				continue
			}

			// If we have a subnet for the device set it back to MAAS
			_, err = c.maasClient.GetSubObject(ifaceUri).Update(url.Values{
				"name": []string{"ma1"},
				"vlan": []string{subnetRec.vlanID},
			})
			if err != nil {
				log.Errorf("Unable to update interface name of '%s (%s)' : %s",
					rec.Name, rec.MAC, err)
			}
		}
	}
}
