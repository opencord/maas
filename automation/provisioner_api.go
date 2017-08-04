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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ProvisionStatus int

const (
	Pending ProvisionStatus = iota
	Running
	Complete
	Failed
)

func (s ProvisionStatus) String() string {
	switch s {
	case Pending:
		return "PENDING"
	case Running:
		return "RUNNING"
	case Complete:
		return "COMPLETE"
	case Failed:
		return "FAILED"
	}
	return "INVALID TASK STATUS"
}

type ProvisionRecord struct {
	Status    ProvisionStatus `json:"status"`
	Timestamp int64
}

type ProvisionRequest struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Ip   string `json:"ip"`
	Mac  string `json:"mac"`
}

type Provisioner interface {
	Get(id string) (*ProvisionRecord, error)
	Provision(prov *ProvisionRequest) error
	Clear(id string) error
}

type ProvisionerConfig struct {
	Url string
}

func buildUrl(base string, id string) string {
	if strings.HasSuffix(base, "/") {
		return base + id
	}
	return base + "/" + id
}

func NewProvisioner(config *ProvisionerConfig) Provisioner {
	return config
}

func (p *ProvisionerConfig) Get(id string) (*ProvisionRecord, error) {
	resp, err := http.Get(buildUrl(p.Url, id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)

	var record ProvisionRecord

	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		err = decoder.Decode(&record)
		if err != nil {
			return nil, err
		}
		return &record, nil
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf(resp.Status)
	}
}

func (p *ProvisionerConfig) Provision(prov *ProvisionRequest) error {
	hc := http.Client{}
	data, err := json.Marshal(prov)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", p.Url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Unexpected response : %s", resp.Status)
	}
	return nil
}

func (p *ProvisionerConfig) Clear(id string) error {
	hc := http.Client{}
	req, err := http.NewRequest("DELETE", buildUrl(p.Url, id), nil)
	if err != nil {
		return err
	}

	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected response : %s", resp.Status)
	}
	return nil
}
