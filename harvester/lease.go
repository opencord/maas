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
	"fmt"
	"net"
	"strings"
	"time"
)

// BindingState type used to maintain lease state
type BindingState uint

// constant values of lease binding state
const (
	Unknown   BindingState = 0
	Free      BindingState = 1
	Active    BindingState = 2
	Expired   BindingState = 3
	Released  BindingState = 4
	Abandoned BindingState = 5
	Reset     BindingState = 6
	Backup    BindingState = 7
	Reserved  BindingState = 8
	Bootp     BindingState = 9
)

// String return a string value for a lease binding state
func (s *BindingState) String() string {
	switch *s {
	case 1:
		return "Free"
	case 2:
		return "Active"
	case 3:
		return "Expired"
	case 4:
		return "Released"
	case 5:
		return "Abandoned"
	case 6:
		return "Reset"
	case 7:
		return "Backup"
	case 8:
		return "Reserved"
	case 9:
		return "Bootp"
	default:
		return "Unknown"
	}
}

// Lease DHCP lease information
type Lease struct {
	BindingState    BindingState     `json:"binding-state"`
	IPAddress       net.IP           `json:"ip-address"`
	ClientHostname  string           `json:"client-hostname"`
	HardwareAddress net.HardwareAddr `json:"hardware-address"`
	Starts          time.Time        `json:"starts"`
	Ends            time.Time        `json:"ends"`
}

// MarshalJSON custom marshaller for DHCP lease
func (l *Lease) MarshalJSON() ([]byte, error) {

	// a custom marshaller is required because the net.Hardware marshals to a string
	// that is not in the standard MAC address format by default as well as the
	// binding state is marshalled to a human readable string
	type Alias Lease
	return json.Marshal(&struct {
		HardwareAddress string `json:"hardware-address"`
		BindingState    string `json:"binding-state"`
		*Alias
	}{
		HardwareAddress: l.HardwareAddress.String(),
		BindingState:    l.BindingState.String(),
		Alias:           (*Alias)(l),
	})
}

// parseBindingState conversts from a string to a valid binding state constant
func parseBindingState(bindingState string) (BindingState, error) {
	switch strings.ToLower(bindingState) {
	case "free":
		return Free, nil
	case "active":
		return Active, nil
	case "expired":
		return Expired, nil
	case "released":
		return Released, nil
	case "abandoned":
		return Abandoned, nil
	case "reset":
		return Reset, nil
	case "backup":
		return Backup, nil
	case "reserved":
		return Reserved, nil
	case "bootp":
		return Bootp, nil
	case "unknown":
		fallthrough
	default:
		return Unknown, nil
	}

	return 0, fmt.Errorf("Unknown lease binding state '%s'", bindingState)
}
