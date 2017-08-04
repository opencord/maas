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
	"fmt"
	"net/url"
	"strings"
)

type Storage interface {
	Put(id string, update StatusMsg) error
	Get(id string) (*StatusMsg, error)
	Delete(id string) error
	List() ([]StatusMsg, error)
}

func NewStorage(spec string) (Storage, error) {
	conn, err := url.Parse(spec)
	if err != nil {
		return nil, err
	}

	switch strings.ToUpper(conn.Scheme) {
	case "MEMORY":
		return NewMemoryStorage(), nil
	case "CONSUL":
		return NewConsulStorage(spec)
	default:
		return nil, fmt.Errorf("Unknown storage scheme specified, '%s'", conn.Scheme)
	}
}

type MemoryStorage struct {
	Data map[string]StatusMsg
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		Data: make(map[string]StatusMsg),
	}
}

func (s *MemoryStorage) Put(id string, update StatusMsg) error {
	s.Data[id] = update
	return nil
}

func (s *MemoryStorage) Get(id string) (*StatusMsg, error) {
	m, ok := s.Data[id]
	if !ok {
		return nil, nil
	}
	return &m, nil
}

func (s *MemoryStorage) Delete(id string) error {
	delete(s.Data, id)
	return nil
}

func (s *MemoryStorage) List() ([]StatusMsg, error) {
	r := make([]StatusMsg, len(s.Data))
	i := 0
	for _, v := range s.Data {
		r[i] = v
		i += 1
	}
	return r, nil
}
