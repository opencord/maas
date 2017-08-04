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
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
)

func NewAddressSource(spec string) (AddressSource, error) {
	u, err := url.Parse(spec)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "file":
		return NewFileAddressSource(u)
	default:
	}
	return nil, fmt.Errorf("Unknown address source scheme specified '%s'", spec)
}

type AddressRec struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	MAC  string `json:"mac"`
}

type AddressSource interface {
	GetAddresses() ([]AddressRec, error)
}

type FileAddressSource struct {
	Path string
}

func NewFileAddressSource(connect *url.URL) (AddressSource, error) {
	// Validate file exists before returning a source
	if _, err := os.Stat(connect.Path); os.IsNotExist(err) {
		return nil, err
	}
	source := FileAddressSource{}
	source.Path = connect.Path
	return &source, nil
}

func (s *FileAddressSource) GetAddresses() ([]AddressRec, error) {
	// Read the file
	file, err := os.Open(s.Path)
	defer file.Close()
	if err != nil {
		return nil, err
	}

	capacity := 20
	result := make([]AddressRec, capacity)
	idx := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())

		// Only process lines with the correct number of parts
		if len(parts) == 6 {
			result[idx].Name = parts[0]
			result[idx].IP = parts[3]
			result[idx].MAC = parts[5]
			idx += 1
			if idx >= capacity {
				capacity += 20
				var tmp []AddressRec
				tmp, result = result, make([]AddressRec, capacity)
				copy(result, tmp)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result[:idx], nil
}
