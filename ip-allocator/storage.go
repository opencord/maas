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
	"math"
	"net"
	"strconv"
	"strings"
)

type Storage interface {
	Init(networkIn string, low string, high string) error
	Get(mac string) (string, error)
	GetAll() map[string]string
	Put(mac, ip string) error
	Remove(mac string) (string, error)
	Dequeue() (string, error)
	Enqueue(ip string) error
}

type MemoryStorage struct {
	allocated               map[string]IPv4
	available               []IPv4
	readIdx, writeIdx, size uint
}

func inIPRange(from net.IP, to net.IP, test net.IP) bool {
	if from == nil || to == nil || test == nil {
		return false
	}

	from16 := from.To16()
	to16 := to.To16()
	test16 := test.To16()
	if from16 == nil || to16 == nil || test16 == nil {
		return false
	}

	if bytes.Compare(test16, from16) >= 0 && bytes.Compare(test16, to16) <= 0 {
		return true
	}
	return false
}

func (s *MemoryStorage) Init(networkIn string, low string, high string) error {
	_, network, err := net.ParseCIDR(networkIn)
	if err != nil {
		return err
	}
	start, _, err := net.ParseCIDR(network.String())
	if err != nil {
		return err
	}

	parts := strings.Split(network.String(), "/")
	ip, err := ParseIP(start.String())
	if err != nil {
		return err
	}
	bits, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	hostCount := int(math.Pow(2, float64(32-bits)))
	s.readIdx = 0
	s.writeIdx = 0
	s.size = uint(hostCount)
	s.allocated = make(map[string]IPv4)
	s.available = make([]IPv4, 0, hostCount)
	ipLow := net.ParseIP(low)
	ipHigh := net.ParseIP(high)
	for i := 0; i < hostCount; i += 1 {
		if inIPRange(ipLow, ipHigh, net.ParseIP(ip.String())) {
			s.available = append(s.available, ip)
		}
		ip, err = ip.Next()
		if err != nil {
			return err
		}
	}
	log.Debugf("AVAILABLE: %+v\n", s.available)
	return nil
}

func (s *MemoryStorage) Get(mac string) (string, error) {
	ip, ok := s.allocated[mac]
	if !ok {
		return "", nil
	}
	return ip.String(), nil
}

func (s *MemoryStorage) GetAll() map[string]string {
	all := make(map[string]string)
	for k, v := range s.allocated {
		all[k] = v.String()
	}
	return all
}

func (s *MemoryStorage) Put(mac, ip string) error {
	data, err := ParseIP(ip)
	if err != nil {
		return err
	}
	s.allocated[mac] = data
	return nil
}

func (s *MemoryStorage) Remove(mac string) (string, error) {
	ip, ok := s.allocated[mac]
	if !ok {
		return "", nil
	}
	delete(s.allocated, mac)
	return ip.String(), nil
}

func (s *MemoryStorage) Dequeue() (string, error) {
	ip := s.available[s.readIdx]
	s.readIdx = (s.readIdx + 1) % s.size
	return ip.String(), nil
}

func (s *MemoryStorage) Enqueue(ip string) error {
	data, err := ParseIP(ip)
	if err != nil {
		return err
	}
	s.available[s.writeIdx] = data
	s.writeIdx = (s.writeIdx + 1) % s.size
	return nil
}
