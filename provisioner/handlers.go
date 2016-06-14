package main

import (
	"bufio"
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !c.validateData(&info) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	role, err := c.GetRole(&info)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = c.dispatcher.Dispatch(&info, role, c.config.Script)
	if err != nil {
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

func (c *Context) QueryStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, ok := vars["nodeid"]
	if !ok || strings.TrimSpace(id) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s, err := c.storage.Get(id)
	if err != nil {
		log.Printf("[warn] Error while retrieving status for '%s' from strorage : %s", id, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if s == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	bytes, err := json.Marshal(s)
	if err != nil {
		log.Printf("[error] Error while attempting to marshal status for '%s' from storage : %s", id, err)
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
