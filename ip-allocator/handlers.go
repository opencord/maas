package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

type ErrorMsg struct {
	Error string
}

type AllocationMsg struct {
	Mac string
	Ip  string
}

func (c *Context) release(mac string, w http.ResponseWriter) {
	err := Release(c.storage, mac)
	if err != nil {
		msg := ErrorMsg{
			Error: err.Error(),
		}
		bytes, err := json.Marshal(&msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, string(bytes), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Context) ReleaseAllocationHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mac := vars["mac"]
	c.release(mac, w)
}

func (c *Context) AllocationHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mac := vars["mac"]

	ip, err := Allocate(c.storage, mac)
	if err != nil {
		msg := ErrorMsg{
			Error: err.Error(),
		}
		bytes, err := json.Marshal(&msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, string(bytes), http.StatusInternalServerError)
		return
	}

	msg := AllocationMsg{
		Mac: mac,
		Ip:  ip,
	}
	bytes, err := json.Marshal(&msg)
	if err != nil {
		msg := ErrorMsg{
			Error: err.Error(),
		}
		bytes, err := json.Marshal(&msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, string(bytes), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

func (c *Context) ListAllocationsHandler(w http.ResponseWriter, r *http.Request) {
	all := c.storage.GetAll()

	list := make([]AllocationMsg, len(all))
	i := 0
	for k, v := range all {
		list[i].Mac = k
		list[i].Ip = v
		i += 1
	}

	bytes, err := json.Marshal(&list)
	if err != nil {
		msg := ErrorMsg{
			Error: err.Error(),
		}
		bytes, err := json.Marshal(&msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, string(bytes), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

func (c *Context) FreeAddressHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ip := vars["ip"]

	all := c.storage.GetAll()
	for k, v := range all {
		if v == ip {
			c.release(k, w)
			return
		}
	}
	http.Error(w, "", http.StatusNotFound)
}
