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
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
)

type Config struct {
	Port             string `default:"8181"`
	IP               string `default:"127.0.0.1"`
	SwitchCount      int    `default:"4"`
	HostCount        int    `default:"4"`
	Username         string `default:"karaf"`
	Password         string `default:"karaf"`
	LogLevel         string `default:"warning" envconfig:"LOG_LEVEL"`
	LogFormat        string `default:"text" envconfig:"LOG_FORMAT"`
	ConfigServerPort string `default:"1337"`
	ConfigServerIP   string `default:"127.0.0.1"`
}

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
	Links []struct {
		Src struct {
			Port   string `json:"port"`
			Device string `json:"device"`
		} `json:"src"`
		Dst struct {
			Port   string `json:"port"`
			Device string `json:"device"`
		} `json:"dst"`
	} `json:"links"`
}

type linkStructJSON struct {
	Val   string
	Comma string
}

type ConfigParam struct {
	SwitchCount int `json:"switchcount"`
	HostCount   int `json:"hostcount"`
}

var c Config

func main() {

	err := envconfig.Process("CONFIGGEN", &c)
	if err != nil {
		log.Fatalf("[ERROR] Unable to parse configuration options : %s", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/config/", ConfigGenHandler).Methods("POST")
	http.Handle("/", router)

	fmt.Println("Config Generator server listening at: " + c.ConfigServerIP + ":" + c.ConfigServerPort)

	http.ListenAndServe(c.ConfigServerIP+":"+c.ConfigServerPort, nil)

}

func ConfigGenHandler(w http.ResponseWriter, r *http.Request) {
	var para ConfigParam

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	if err := decoder.Decode(&para); err != nil {
		fmt.Errorf("Unable to decode request to provision : %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.HostCount = para.HostCount
	c.SwitchCount = para.SwitchCount

	onos := "http://" + c.Username + ":" + c.Password + "@" + c.IP + ":" + c.Port

	err := os.Remove("network-cfg.json")
	if err != nil {
		log.Println("Warning: no file called network-cfg.json (ignore if this is the first run)")
	}
	err = generateDevicesJSON(onos)
	if err != nil {
		w.WriteHeader(http.StatusExpectationFailed)
		fmt.Fprintf(w, err.Error())
		return
	}
	generateLinkJSON(onos)
	err = generateHostJSON(onos)
	if err != nil {
		w.WriteHeader(http.StatusExpectationFailed)
		fmt.Fprintf(w, err.Error())
		return
	}

	fmt.Println("Config file generated: network-cfg.json")

	data, err := ioutil.ReadFile("network-cfg.json")
	check(err)

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, string(data))

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

func generateDevicesJSON(onos string) error {
	ds := getData(onos + "/onos/v1/devices")

	var d devices
	err := json.Unmarshal(ds, &d)
	check(err)

	if len(d.Device) != c.SwitchCount {
		_ = os.Remove("network-cfg.json")
		log.Println("[INFO] Cleaning up unfinished config file")
		e := fmt.Sprintf("[ERROR] Number of switches configured don't match actual switches connected to the controller. Configured: %d, connected: %d", c.SwitchCount, len(d.Device))
		log.Println(e)
		return errors.New(e)
	}

	for k, _ := range d.Device {
		d.Device[k].Comma = ","
		if k >= len(d.Device)-1 {
			d.Device[k].Comma = ""
		}
	}

	writeToFile(d.Device, "devices.tpl")
	return nil

}

func generateHostJSON(onos string) error {
	hs := getData(onos + "/onos/v1/hosts")
	var h hosts
	err := json.Unmarshal(hs, &h)
	check(err)

	if len(h.Host) != c.HostCount {
		_ = os.Remove("network-cfg.json")
		log.Println("[INFO] Cleaning up unfinished config file")
		e := fmt.Sprintf("[ERROR] Number of hosts configured don't match actual hosts visible to the controller. Configured: %d, connected: %d", c.HostCount, len(h.Host))
		log.Println(e)
		return errors.New(e)
	}

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
	return nil

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
