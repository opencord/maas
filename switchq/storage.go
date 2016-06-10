package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"
)

func NewStorage(spec string) (Storage, error) {
	u, err := url.Parse(spec)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "memory":
		return NewMemoryStorage(u)
	default:
	}
	return nil, fmt.Errorf("Unknown storage scheme specified, '%s'", u.Scheme)
}

type Storage interface {
	Switchq(mac string) (bool, error)
	LastMACCheck(mac string) (*time.Time, error)
	MarkMACCheck(mac string, when *time.Time) error
	LastProvisioned(mac string) (*time.Time, error)
	MarkProvisioned(mac string, when *time.Time) error
}

type VendorRec struct {
	Prefix    string `json:"prefix"`
	Vendor    string `json:"vendor"`
	Provision bool   `json:"provision"`
}

type MemoryStorage struct {
	Vendors map[string]VendorRec
	Checks  map[string]time.Time
	Times   map[string]time.Time
}

func NewMemoryStorage(u *url.URL) (Storage, error) {

	s := MemoryStorage{}
	s.Vendors = make(map[string]VendorRec)

	if u.Path != "" {
		file, err := os.Open(u.Path)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		data := make([]VendorRec, 0)
		decoder := json.NewDecoder(file)
		err = decoder.Decode(&data)
		if err != nil {
			return nil, err
		}
		for _, rec := range data {
			s.Vendors[rec.Prefix] = rec
		}
		log.Printf("[debug] %v", s.Vendors)

	} else {
		log.Printf("[warn] no vendors have been set, no switches will be provisioned")
	}
	return &s, nil
}

func (s *MemoryStorage) Switchq(mac string) (bool, error) {
	if len(mac) < 8 {
		return false, nil
	}
	rec, ok := s.Vendors[strings.ToUpper(mac[0:8])]
	if !ok || !rec.Provision {
		return false, nil
	}

	return true, nil
}

func (s *MemoryStorage) LastMACCheck(mac string) (*time.Time, error) {
	when, ok := s.Checks[mac]
	if !ok {
		return nil, nil
	}
	result := when
	return &result, nil
}

func (s *MemoryStorage) MarkMACCheck(mac string, when *time.Time) error {
	s.Checks[mac] = *when
	return nil
}

func (s *MemoryStorage) LastProvisioned(mac string) (*time.Time, error) {
	when, ok := s.Times[mac]
	if !ok {
		return nil, nil
	}
	result := when
	return &result, nil
}

func (s *MemoryStorage) MarkProvisioned(mac string, when *time.Time) error {
	s.Times[mac] = *when
	return nil
}
