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
	"net/http"
	"strings"
	"text/template"
)

type GenerationOptions struct {
	SwitchCount int `json:"switchcount"`
	HostCount   int `json:"hostcount"`
}

func (c *Config) configGenHandler(w http.ResponseWriter, r *http.Request) {
	var options GenerationOptions

	deviceMap := make(map[string]*onosDevice)

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	if err := decoder.Decode(&options); err != nil {
		log.Errorf("Unable to decode provisioning request options: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var devices onosDevices
	err := c.fetch("/onos/v1/devices", &devices)
	if err != nil {
		log.Errorf("Unable to retrieve device information from controller: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If the request specified the number of switches, validate we have that
	// exact number
	if options.SwitchCount > 0 && len(devices.Devices) != options.SwitchCount {
		log.Errorf("Expecting %d switch(es), found %d, no configuration generated",
			options.SwitchCount, len(devices.Devices))
		http.Error(w, "Expected switch count mismatch",
			http.StatusInternalServerError)
		return
	}

	for _, device := range devices.Devices {
		deviceMap[device.Id] = device
		device.Mac = splitString(device.ChassisId, 2, ":")
	}

	var hosts onosHosts
	err = c.fetch("/onos/v1/hosts", &hosts)
	if err != nil {
		log.Errorf("Unable to retrieve host information from controller: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	}

	// If the request specified the number of hosts, validate we have that
	// exact number
	if options.HostCount > 0 && len(hosts.Hosts) != options.HostCount {
		log.Errorf("Expecting %d host(s), found %d, no configuration generated",
			options.HostCount, len(hosts.Hosts))
		http.Error(w, "Expected host count mismatch",
			http.StatusInternalServerError)
		return
	}

	// Use a simple heuristic to determine which switches are edge routers
	// and which are not
	markEdgeRouters(deviceMap, hosts)

	// Generate the configuration file
	cfg := onosConfig{
		Devices: devices.Devices,
		Hosts:   hosts.Hosts,
	}

	funcMap := template.FuncMap{
		// The name "inc" is what the function will be called in the template text.
		"add": func(a, b int) int {
			return a + b
		},
		"gateway": func(ips []string) string {
			// Find the first v4 address, as determined by
			// not having a ':' in the IP
			for _, ip := range ips {
				if strings.Index(ip, ":") == -1 {
					parts := strings.Split(ip, ".")
					targetIp := ""
					for _, v := range parts[:len(parts)-1] {
						targetIp = targetIp + v + "."
					}
					return targetIp + "254/24"
				}
			}
			return "0.0.0.254/24"
		},
		"vlan": func(ips []string) string {
			// Find the first v4 address, as determined by
			// not having a ':' in the IP
			for _, ip := range ips {
				if strings.Index(ip, ":") == -1 {
					return strings.Split(ip, ".")[2]
				}
			}
			return "0"
		},
	}

	tpl, err := template.New("netconfig.tpl").Funcs(funcMap).ParseFiles("netconfig.tpl")
	if err != nil {
		log.Errorf("Unable to parse template: %s", err)
		http.Error(w, "Template parse error", http.StatusInternalServerError)
		return
	}

	// Write template to buffer, so if there is an error we can return an
	// http error
	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, cfg)
	if err != nil {
		log.Errorf("Unexpected error while processing template: %s", err)
		http.Error(w, "Template processing error", http.StatusInternalServerError)
	}

	w.Write(buf.Bytes())
}

// markEdgeRouters use hueristic to determine and mark switches that act
// as edge routers
func markEdgeRouters(dm map[string]*onosDevice, hosts onosHosts) {
	// Walk the list of know compute nodes (hosts) and if the compute node
	// is connected to a switch, then that switch is an edge router
	for _, host := range hosts.Hosts {
		if device, ok := dm[host.Location.ElementID]; ok {
			(*device).IsEdgeRouter = true
		}
	}
}

// splitString used to convert a string to a psudeo MAC address by
// splitting and separating it with a colon
func splitString(src string, n int, sep string) string {
	r := ""
	for i, c := range src {
		if i > 0 && i%n == 0 {
			r += sep
		}
		r += string(c)
	}
	return r
}

// fetch fetch the specified data from ONOS
func (c *Config) fetch(path string, data interface{}) error {
	resp, err := http.Get(c.connect + path)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(data)
	return err
}
