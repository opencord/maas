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
	"encoding/json"
	"net/http"
	"strings"
)

type Vendors interface {
	Switchq(mac string) (bool, error)
}

type VendorRec struct {
	Prefix    string `json:"prefix"`
	Vendor    string `json:"vendor"`
	Provision bool   `json:"provision"`
}

type VendorsData struct {
	Vendors map[string]VendorRec
}

func NewVendors(spec string) (Vendors, error) {
	v := VendorsData{}
	v.Vendors = make(map[string]VendorRec)

	t := &http.Transport{}
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
	c := &http.Client{Transport: t}
	res, err := c.Get(spec)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data := make([]VendorRec, 0)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}
	for _, rec := range data {
		v.Vendors[rec.Prefix] = rec
	}
	log.Debugf("known vendors %+v", v.Vendors)

	return &v, nil
}

func (v *VendorsData) Switchq(mac string) (bool, error) {
	// If there is a "full" MAC then attempt to see if that can
	// be matched. If matched, accept that result and look no
	// further
	if len(mac) == 17 {
		if rec, ok := v.Vendors[strings.ToUpper(mac)]; ok {
			return rec.Provision, nil
		}
	}

	// If we have at least a OUI, look to see if that can be
	// be matched. If matched, accept that result and look no
	// further.
	if len(mac) >= 8 {
		if rec, ok := v.Vendors[strings.ToUpper(mac[0:8])]; ok {
			return rec.Provision, nil
		}
	}

	// No match found, so assume false.
	return false, nil
}
