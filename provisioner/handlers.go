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
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
)

type RequestInfo struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	Ip           string `json:"ip"`
	Mac          string `json:"mac"`
	RoleSelector string `json:"role_selector"`
	Role         string `json:"role"`
	Script       string `json:"script"`
}

func (c *Context) GetRole(info *RequestInfo) (string, error) {
	if info.Role != "" {
		return info.Role, nil
	} else if c.config.RoleSelectorURL == "" && info.RoleSelector == "" {
		return c.config.DefaultRole, nil
	}
	selector := c.config.RoleSelectorURL
	if info.RoleSelector != "" {
		selector = info.RoleSelector
	}

	r, err := http.Get(selector)
	if err != nil {
		return "", err
	}

	s := bufio.NewScanner(r.Body)
	defer r.Body.Close()
	role := strings.TrimSpace(s.Text())
	if role == "" {
		return c.config.DefaultRole, nil
	}
	return role, nil
}

func (c *Context) validateData(info *RequestInfo) bool {
	if strings.TrimSpace(info.Id) == "" ||
		strings.TrimSpace(info.Name) == "" ||
		strings.TrimSpace(info.Ip) == "" ||
		strings.TrimSpace(info.Mac) == "" {
		return false
	}
	return true
}

func (c *Context) ProvisionRequestHandler(w http.ResponseWriter, r *http.Request) {
	var info RequestInfo
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	if err := decoder.Decode(&info); err != nil {
		log.Errorf("Unable to decode request to provision : %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !c.validateData(&info) {
		log.Errorf("Provisioning request not valid for '%s'", info.Name)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	role, err := c.GetRole(&info)
	if err != nil {
		log.Errorf("unable to get provisioning role for node '%s' : %s", info.Name, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If the request has a script set, override the default configuration
	script := c.config.Script
	if info.Script != "" {
		script = info.Script
	}
	err = c.dispatcher.Dispatch(&info, role, script)
	if err != nil {
		log.Errorf("unable to dispatch provisioning request for node '%s' : %s", info.Name, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (c *Context) ListRequestsHandler(w http.ResponseWriter, r *http.Request) {
	list, err := c.storage.List()
	bytes, err := json.Marshal(list)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

func (c *Context) DeleteStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, ok := vars["nodeid"]
	if !ok || strings.TrimSpace(id) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err := c.storage.Delete(id)
	if err != nil {
		log.Errorf("Error while deleting status fo '%s' from storage : %s", id, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *Context) QueryStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, ok := vars["nodeid"]
	if !ok || strings.TrimSpace(id) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s, err := c.storage.Get(id)
	if err != nil {
		log.Errorf("Error while retrieving status for '%s' from strorage : %s", id, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if s == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	bytes, err := json.Marshal(s)
	if err != nil {
		log.Errorf("Error while attempting to marshal status for '%s' from storage : %s", id, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch s.Status {
	case Pending, Running:
		w.WriteHeader(http.StatusAccepted)
	case Failed, Complete:
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Write(bytes)
}
