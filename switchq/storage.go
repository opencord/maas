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
	"fmt"
	"net/url"
	"time"
)

func NewStorage(spec string) (Storage, error) {
	u, err := url.Parse(spec)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "memory":
		return NewMemoryStorage()
	default:
	}
	return nil, fmt.Errorf("Unknown storage scheme specified, '%s'", u.Scheme)
}

type Storage interface {
	LastMACCheck(mac string) (*time.Time, error)
	MarkMACCheck(mac string, when *time.Time) error
	LastProvisioned(mac string) (*time.Time, error)
	MarkProvisioned(mac string, when *time.Time) error
	ClearProvisioned(mac string) error
}

type MemoryStorage struct {
	Checks map[string]time.Time
	Times  map[string]time.Time
}

func NewMemoryStorage() (Storage, error) {

	s := MemoryStorage{
		Checks: make(map[string]time.Time),
		Times:  make(map[string]time.Time),
	}
	return &s, nil
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

func (s *MemoryStorage) ClearProvisioned(mac string) error {
	delete(s.Times, mac)
	return nil
}
