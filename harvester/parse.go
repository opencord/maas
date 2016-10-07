// Copyright 2016 Open Networking Laboratory
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
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// leaseFilterFunc provides a mechanism to filter which leases are returned by lease file parser
type leaseFilterFunc func(lease *Lease) bool

const (
	// returns if a parse requests is processed or denied because of quiet period
	responseQuiet uint = 0
	responseOK    uint = 1

	// time format for parsing time stamps in lease file
	dateTimeLayout = "2006/1/2 15:04:05"

	bindFileFormat = "{{.ClientHostname}}\tIN A {{.IPAddress}}\t; {{.HardwareAddress}}"
)

// parseLease parses a single lease from the lease file
func parseLease(scanner *bufio.Scanner, lease *Lease) error {
	var err error
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 0 {
			switch fields[0] {
			case "}":
				// If no client-hostname was specified, generate one
				if len(lease.ClientHostname) == 0 {
					lease.ClientHostname = strings.ToUpper("UNK-" +
						strings.Replace(lease.HardwareAddress.String(), ":", "", -1))
				}
				return nil
			case "client-hostname":
				lease.ClientHostname = strings.Trim(fields[1], "\";")
			case "hardware":
				lease.HardwareAddress, err = net.ParseMAC(strings.Trim(fields[2], ";"))
				if err != nil {
					return err
				}
			case "binding":
				lease.BindingState, err = parseBindingState(strings.Trim(fields[2], ";"))
				if err != nil {
					return err
				}
			case "starts":
				lease.Starts, err = time.Parse(dateTimeLayout,
					fields[2]+" "+strings.Trim(fields[3], ";"))
				if err != nil {
					return err
				}
			case "ends":
				lease.Ends, err = time.Parse(dateTimeLayout,
					fields[2]+" "+strings.Trim(fields[3], ";"))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// parseLeaseFile parses the entire lease file
func parseLeaseFile(filename string, filterFunc leaseFilterFunc) (map[string]*Lease, error) {
	leases := make(map[string]*Lease)

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 0 && fields[0] == "lease" {
			lease := Lease{}
			lease.IPAddress = net.ParseIP(fields[1])
			parseLease(scanner, &lease)
			if filterFunc(&lease) {
				leases[lease.IPAddress.String()] = &lease
			}
		}
	}

	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return leases, nil
}

// syncRequestHandler accepts requests to parse the lease file and either processes or ignores because of quiet period
func (app *application) syncRequestHandler(requests chan *chan uint) {

	// track the last time file was processed to enforce quiet period
	var last *time.Time = nil

	// process requests on the channel
	for response := range requests {
		now := time.Now()

		// if the request is made during the quiet period then drop the request to prevent
		// a storm
		if last != nil && now.Sub(*last) < app.QuietPeriod {
			app.log.Warn("Request received during query quiet period, will not harvest")
			if response != nil {
				*response <- responseQuiet
			}
			continue
		}

		// process the lease database
		app.log.Infof("Synchronizing DHCP lease database")
		leases, err := parseLeaseFile(app.DHCPLeaseFile,
			func(lease *Lease) bool {
				return lease.BindingState != Free &&
					lease.Ends.After(now) &&
					lease.Starts.Before(now)
			})
		if err != nil {
			app.log.Errorf("Unable to parse DHCP lease file at '%s' : %s",
				app.DHCPLeaseFile, err)
		} else {
			// if configured to verify leases with a ping do so
			if app.VerifyLeases {
				app.log.Infof("Verifing %d discovered leases", len(leases))
				_, err := app.verifyLeases(leases)
				if err != nil {
					app.log.Errorf("unexpected error while verifing leases : %s", err)
					app.log.Infof("Discovered %d active, not verified because of error, DHCP leases",
						len(leases))
				} else {
					app.log.Infof("Discovered %d active and verified DHCP leases", len(leases))
				}
			} else {
				app.log.Infof("Discovered %d active, not not verified, DHCP leases", len(leases))
			}

			// if configured to output the lease information to a file, do so
			if len(app.OutputFile) > 0 {
				app.log.Infof("Writing lease information to file '%s'", app.OutputFile)
				out, err := os.Create(app.OutputFile)
				if err != nil {
					app.log.Errorf(
						"unexpected error while attempting to open file `%s' for output : %s",
						app.OutputFile, err)
				} else {
					table := tabwriter.NewWriter(out, 1, 0, 4, ' ', 0)
					for _, lease := range leases {
						if err := app.outputTemplate.Execute(table, lease); err != nil {
							app.log.Errorf(
								"unexpected error while writing leases to file '%s' : %s",
								app.OutputFile, err)
							break
						}
						fmt.Fprintln(table)
					}
					table.Flush()
				}
				out.Close()
			}

			// if configured to reload the DNS server, then use the RNDC command to do so
			if app.RNDCUpdate {
				cmd := exec.Command("rndc", "-s", app.RNDCAddress, "-p", strconv.Itoa(app.RNDCPort),
					"-c", app.RNDCKeyFile, "reload", app.RNDCZone)
				err := cmd.Run()
				if err != nil {
					app.log.Errorf("Unexplected error while attempting to reload zone '%s' on DNS server '%s:%d' : %s", app.RNDCZone, app.RNDCAddress, app.RNDCPort, err)
				} else {
					app.log.Infof("Successfully reloaded DNS zone '%s' on server '%s:%d' via RNDC command",
						app.RNDCZone, app.RNDCAddress, app.RNDCPort)
				}
			}

			// process the results of the parse to internal data structures
			app.interchange.Lock()
			app.leases = leases
			app.byHostname = make(map[string]*Lease)
			app.byHardware = make(map[string]*Lease)
			for _, lease := range leases {
				app.byHostname[lease.ClientHostname] = lease
				app.byHardware[lease.HardwareAddress.String()] = lease
			}
			leases = nil
			app.interchange.Unlock()
		}
		if last == nil {
			last = &time.Time{}
		}
		*last = time.Now()

		if response != nil {
			*response <- responseOK
		}
	}
}

// syncFromDHCPLeaseFileLoop periodically request a lease file processing
func (app *application) syncFromDHCPLeaseFileLoop(requests chan *chan uint) {
	responseChan := make(chan uint)
	for {
		requests <- &responseChan
		select {
		case _ = <-responseChan:
			// request completed
		case <-time.After(app.RequestTimeout):
			app.log.Error("request to process DHCP lease file timed out")
		}
		time.Sleep(app.QueryPeriod)
	}
}
