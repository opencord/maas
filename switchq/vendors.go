package main

import (
	"encoding/json"
	"log"
	"strings"
	"net/http"
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
	log.Printf("[debug] %v", v.Vendors)

	return &v, nil
}

func (v *VendorsData) Switchq(mac string) (bool, error) {
	if len(mac) < 8 {
		return false, nil
	}
	rec, ok := v.Vendors[strings.ToUpper(mac[0:8])]
	if !ok || !rec.Provision {
		return false, nil
	}

	return true, nil
}
