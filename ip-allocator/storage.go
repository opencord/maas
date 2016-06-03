package main

import (
	"math"
	"net"
	"strconv"
	"strings"
)

type Storage interface {
	Init(networkIn string, skip int) error
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

func (s *MemoryStorage) Init(networkIn string, skip int) error {
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
	hostCount := int(math.Pow(2, float64(32-bits))) - skip
	s.readIdx = 0
	s.writeIdx = 0
	s.size = uint(hostCount)
	s.allocated = make(map[string]IPv4)
	s.available = make([]IPv4, hostCount)
	for i := 0; i < skip; i += 1 {
		ip, err = ip.Next()
		if err != nil {
			return err
		}
	}
	for i := 0; i < hostCount; i += 1 {
		s.available[i] = ip
		ip, err = ip.Next()
		if err != nil {
			return err
		}
	}
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
