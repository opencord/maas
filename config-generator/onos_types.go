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

type onosHost struct {
	Id          string   `json:"id"`
	Mac         string   `json:"mac"`
	IpAddresses []string `json:"ipAddresses"`
	Location    struct {
		ElementID string `json:"elementId`
		Port      string `json:"port"`
	} `json:"location"`
}

type onosHosts struct {
	Hosts []*onosHost `json:"hosts"`
}

type onosDevice struct {
	Id           string `json:"id"`
	ChassisId    string `json:"chassisId"`
	IsEdgeRouter bool   `json:"isEdgeRouter"`
	Annotations  struct {
		ManagementAddress string `json:"managementAddress"`
	} `json:"annotations"`
	Mac string `json:"-"`
}

type onosDevices struct {
	Devices []*onosDevice `json:"devices"`
}

type onosLink struct {
	Src struct {
		Port   string `json:"port"`
		Device string `json:"device"`
	} `json:"src"`
	Dst struct {
		Port   string `json:"port"`
		Device string `json:"device"`
	} `json:"dst"`
}

type onosLinks struct {
	Links []*onosLink `json:"links"`
}

type onosConfig struct {
	Devices []*onosDevice
	Hosts   []*onosHost
	Links   []*onosLink
}
