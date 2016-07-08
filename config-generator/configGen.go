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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/template"
)

type hosts struct {
	Host []struct {
		Mac         string   `json:"mac"`
		IpAddresses []string `json:"ipAddresses"`
		Location    struct {
			ElementID string `json:"elementId`
			Port      string `json:"port"`
		} `json:"location"`
		Comma   string
		Gateway string
	} `json:"hosts"`
}

type devices struct {
	Device []struct {
		Id          string `json:"id"`
		ChassisId   string `json:"chassisId"`
		Annotations struct {
			ManagementAddress string `json:"managementAddress"`
		} `json:"annotations"`
		Comma string `default:","`
	} `json:"devices"`
}

type onosLinks struct {
	Links []link `json:"links"`
}

type link struct {
	Src devicePort `json:"src"`
	Dst devicePort `json:"dst"`
}

type devicePort struct {
	Port   string `json:"port"`
	Device string `json:"device"`
}

type linkStructJSON struct {
	Val   string
	Comma string
}

func main() {
	onos := "http://karaf:karaf@127.0.0.1:8181"

	err := os.Remove("network-cfg.json")
	if err != nil {
		fmt.Println("Warning: no file called network-cfg.json (ignore if this is the first run)")
	}
	generateDevicesJSON(onos)
	generateLinkJSON(onos)
	generateHostJSON(onos)

	fmt.Println("Config file generated: network-cfg.json")

}

func writeToFile(object interface{}, t string) {
	f, err := os.OpenFile("network-cfg.json", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	tpl, err := template.ParseFiles(t)
	check(err)
	err = tpl.Execute(f, object)
	check(err)
}

func generateDevicesJSON(onos string) {
	ds := getData(onos + "/onos/v1/devices")

	var d devices
	err := json.Unmarshal(ds, &d)
	check(err)

	for k, _ := range d.Device {
		d.Device[k].Comma = ","
		if k >= len(d.Device)-1 {
			d.Device[k].Comma = ""
		}
	}

	writeToFile(d.Device, "devices.tpl")

}

func generateHostJSON(onos string) {
	hs := getData(onos + "/onos/v1/hosts")
	var h hosts
	err := json.Unmarshal(hs, &h)
	check(err)

	for k, _ := range h.Host {

		h.Host[k].Comma = ","
		if k >= len(h.Host)-1 {
			h.Host[k].Comma = ""
		}

		parts := strings.Split(h.Host[k].IpAddresses[0], ".")
		ip := ""
		for _, v := range parts[:len(parts)-1] {
			ip = ip + v + "."
		}
		h.Host[k].Gateway = ip
	}

	writeToFile(h.Host, "ports.tpl")

	writeToFile(h.Host, "hosts.tpl")

}

func generateLinkJSON(onos string) {

	links := getData(onos + "/onos/v1/links")

	var l onosLinks
	err := json.Unmarshal(links, &l)
	check(err)

	var in []linkStructJSON

	for k, v := range l.Links {

		comma := ","
		val := fmt.Sprint(v.Src.Device + "/" + v.Src.Port + "-" + v.Dst.Device + "/" + v.Dst.Port)
		if k >= len(l.Links)-1 {
			comma = ""
		}

		tmp := linkStructJSON{val, comma}
		in = append(in, tmp)

	}

	writeToFile(in, "links.tpl")

}

func getData(url string) []byte {

	resp, err := http.Get(url)
	check(err)

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	check(err)

	return body

}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
