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
	"github.com/tatsushid/go-fastping"
	"net"
	"time"
)

// NOTE: the go-fastping utility calls its handlers (OnRecv, OnIdle) from a single thread as such
// the code below does not have to serialize access to the "nonverified" array as access is
// serialized the the fastping utility calling from a single thread.

// verifyLeases verifies that the lease is valid by using an ICMP ping
func (app *application) verifyLeases(leases map[string]*Lease) (map[string]*Lease, error) {
	nonverified := make(map[string]bool)

	// Populate the non-verified list from all the leases and then we will remove those
	// that are verified
	for ip, _ := range leases {
		nonverified[ip] = true
	}

	pinger := fastping.NewPinger()
	for _, lease := range leases {
		pinger.AddIPAddr(&net.IPAddr{IP: lease.IPAddress})
	}

	if app.VerifyWithUDP {
		pinger.Network("udp")
	}
	pinger.MaxRTT = app.VerifyTimeout

	// when a ping response is received remove that lease from the non-verified list
	pinger.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		app.log.Infof("Verified lease for IP address '%s' with RTT of '%s'", addr.String(), rtt)
		delete(nonverified, addr.String())
	}
	err := pinger.Run()
	if err != nil {
		return nil, err
	}

	// Remove unverified leases from list
	for ip, _ := range nonverified {
		app.log.Infof("Discarding lease for IP address '%s', could not be verified", ip)
		delete(leases, ip)
	}

	return leases, nil
}
