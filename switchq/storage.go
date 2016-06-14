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
