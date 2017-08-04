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
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
	"time"
)

// listLeaseHandler returns a list of all known leases
func (app *application) listLeasesHandler(w http.ResponseWriter, r *http.Request) {

	// convert data map of leases to a slice
	app.interchange.RLock()
	leases := make([]Lease, len(app.leases))
	i := 0
	for _, lease := range app.leases {
		leases[i] = *lease
		i += 1
	}
	app.interchange.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(leases)
}

// getLeaseHandler return a single known lease
func (app *application) getLeaseHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ip, ok := vars["ip"]
	if !ok || strings.TrimSpace(ip) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	app.interchange.RLock()
	lease, ok := app.leases[ip]
	app.interchange.RUnlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(lease)
}

// getLeaseByHardware return a single known lease by its MAC address
func (app *application) getLeaseByHardware(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mac, ok := vars["mac"]
	if !ok || strings.TrimSpace(mac) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	app.interchange.RLock()
	lease, ok := app.byHardware[mac]
	app.interchange.RUnlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(lease)
}

// getLeaseByHostname return a single known lease by its hostname
func (app *application) getLeaseByHostname(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok || strings.TrimSpace(name) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	app.interchange.RLock()
	lease, ok := app.byHostname[name]
	app.interchange.RUnlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(lease)
}

// doHarvestHandler request a harvest of lease information and return if it was completed or during the quiet period
func (app *application) doHarvestHandler(w http.ResponseWriter, r *http.Request) {
	app.log.Info("Manual harvest invocation")
	responseChan := make(chan uint)
	app.requests <- &responseChan
	select {
	case response := <-responseChan:
		switch response {
		case responseOK:
			w.Header().Set("Content-Type", "application/json")
			encoder := json.NewEncoder(w)
			encoder.Encode(struct {
				Response string `json:"response"`
			}{
				Response: "OK",
			})
		case responseQuiet:
			w.Header().Set("Content-Type", "application/json")
			encoder := json.NewEncoder(w)
			encoder.Encode(struct {
				Response string `json:"response"`
			}{
				Response: "QUIET",
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	case <-time.After(app.RequestTimeout):
		app.log.Error("Request to process DHCP lease file timed out")
		w.WriteHeader(http.StatusInternalServerError)
	}
}
