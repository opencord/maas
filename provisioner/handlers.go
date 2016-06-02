package main

import (
	"bufio"
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
)

type RequestInfo struct {
	Id   string
	Name string
	Ip   string
	Mac  string
}

func (c *Context) GetRole(info *RequestInfo) (string, error) {
	if c.config.RoleSelectorURL == "" {
		return c.config.DefaultRole, nil
	}
	r, err := http.Get(c.config.RoleSelectorURL)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if s == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	bytes, err := json.Marshal(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}
