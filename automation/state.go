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
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	maas "github.com/juju/gomaasapi"
)

// Action how to get from there to here
type Action func(*maas.MAASObject, MaasNode, ProcessingOptions) error

// Transition the map from where i want to be from where i might be
type Transition struct {
	Target  string
	Current string
	Using   Action
}

type Power struct {
	Name          string `json:"name"`
	MacAddress    string `json:"mac_address"`
	PowerPassword string `json:"power_password"`
	PowerAddress  string `json:"power_address"`
}

type HostFilter struct {
	Zones struct {
		Include []string `json:"include,omitempty"`
		Exclude []string `json:"exclude,omitempty"`
	} `json:"zones,omitempty"`
	Hosts struct {
		Include []string `json:"include,omitempty"`
		Exclude []string `json:"exclude,omitempty"`
	} `json:"hosts,omitempty"`
}

// ProcessingOptions used to determine on what hosts to operate
type ProcessingOptions struct {
	Filter          HostFilter
	Mappings        map[string]string
	Preview         bool
	AlwaysRename    bool
	Provisioner     Provisioner
	ProvisionURL    string
	ProvisionTTL    time.Duration
	PowerHelper     string
	PowerHelperUser string
	PowerHelperHost string
}

// Transitions the actual map
//
// Currently this is a hand compiled / optimized "next step" table. This should
// really be generated from the state machine chart input. Once this has been
// accomplished you should be able to determine the action to take given your
// target state and your current state.
var Transitions = map[string]map[string][]Action{
	"Deployed": {
		"New":                 []Action{Reset, Commission},
		"Deployed":            []Action{Provision, Done},
		"Ready":               []Action{Reset, Aquire},
		"Allocated":           []Action{Reset, Deploy},
		"Retired":             []Action{Reset, AdminState},
		"Reserved":            []Action{Reset, AdminState},
		"Releasing":           []Action{Reset, Wait},
		"DiskErasing":         []Action{Reset, Wait},
		"Deploying":           []Action{Reset, Wait},
		"Commissioning":       []Action{Reset, Wait},
		"Missing":             []Action{Reset, Fail},
		"FailedReleasing":     []Action{Reset, Fail},
		"FailedDiskErasing":   []Action{Reset, Fail},
		"FailedDeployment":    []Action{Reset, Fail},
		"Broken":              []Action{Reset, Fail},
		"FailedCommissioning": []Action{Reset, Fail},
	},
}

const (
	// defaultStateMachine Would be nice to drive from a graph language
	defaultStateMachine string = `
        (New)->(Commissioning)
        (Commissioning)->(FailedCommissioning)
        (FailedCommissioning)->(New)
        (Commissioning)->(Ready)
        (Ready)->(Deploying)
        (Ready)->(Allocated)
        (Allocated)->(Deploying)
        (Deploying)->(Deployed)
        (Deploying)->(FailedDeployment)
        (FailedDeployment)->(Broken)
        (Deployed)->(Releasing)
        (Releasing)->(FailedReleasing)
        (FailedReleasing)->(Broken)
        (Releasing)->(DiskErasing)
        (DiskErasing)->(FailedEraseDisk)
        (FailedEraseDisk)->(Broken)
        (Releasing)->(Ready)
        (DiskErasing)->(Ready)
        (Broken)->(Ready)
        (Deployed)->(Provisioning)
        (Provisioning)->(HTTP PUT)
        (HTTP PUT)->(HTTP GET)
        (HTTP GET)->(HTTP GET)
        (HTTP GET)->|b|
        |b|->(Provisioned)
        |b|->(ProvisionError)
        (ProvisionError)->(Provisioning)`
)

// updateName - changes the name of the MAAS node based on the configuration file
func updateNodeName(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	macs := node.MACs()

	// Get current node name and strip off domain name
	current := node.Hostname()
	if i := strings.IndexRune(current, '.'); i != -1 {
		current = current[:i]
	}
	for _, mac := range macs {
		if name, ok := options.Mappings[mac]; ok {
			if current != name {
				nodesObj := client.GetSubObject("nodes")
				nodeObj := nodesObj.GetSubObject(node.ID())
				log.Infof("RENAME '%s' to '%s'\n", node.Hostname(), name)

				if !options.Preview {
					nodeObj.Update(url.Values{"hostname": []string{name}})
				}
			}
		}
	}
	return nil
}

// Reset we are at the target state, nothing to do
var Reset = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Debugf("RESET: %s", node.Hostname())

	if options.AlwaysRename {
		updateNodeName(client, node, options)
	}

	err := options.Provisioner.Clear(node.ID())
	if err != nil {
		log.Errorf("Attempting to clear provisioning state of node '%s' : %s", node.ID(), err)
	}
	return err
}

// Provision we are at the target state, nothing to do
var Provision = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Debugf("CHECK PROVISION: %s", node.Hostname())

	if options.AlwaysRename {
		updateNodeName(client, node, options)
	}

	record, err := options.Provisioner.Get(node.ID())
	if err != nil {
		log.Warningf("unable to retrieve provisioning state of node '%s' : %s", node.Hostname(), err)
	} else if record == nil || record.Status == Failed {
		var label string
		if record == nil {
			label = "NotFound"
		} else {
			label = record.Status.String()
		}
		log.Debugf("Current state of node '%s' is '%s'", node.Hostname(), label)
		ips := node.IPs()
		ip := ""
		if len(ips) > 0 {
			ip = ips[0]
		} else {
			// An IP is required by the provisioner, so if we don't have one then attempt
			// to resolve the name
			log.Debugf("MAAS did not return the IP address of host '%s', attempting to resolve independently",
				node.Hostname())
			addrs, err := net.LookupHost(node.Hostname())
			if err != nil || len(addrs) == 0 {
				log.Errorf("Unable to determine IP address of '%s', thus unable to provision node '%s'",
					node.Hostname(), node.ID())
				if err == nil {
					err = fmt.Errorf("Unable to determine IP address of host '%s'", node.Hostname)
				} else {
					err = fmt.Errorf("Unable to determine IP address of host '%s' : %s",
						node.Hostname, err)
				}
				return err
			}
			ip = addrs[0]
			log.Debugf("Resolved hostname '%s' to IP address '%s'", node.Hostname(), ip)
		}
		macs := node.MACs()
		mac := ""
		if len(macs) > 0 {
			mac = macs[0]
		}
		log.Debugf("POSTing '%s' (%s) to '%s'", node.Hostname(), node.ID(), options.ProvisionURL)
		err = options.Provisioner.Provision(&ProvisionRequest{
			Id:   node.ID(),
			Name: node.Hostname(),
			Ip:   ip,
			Mac:  mac,
		})

		if err != nil {
			log.Errorf("unable to provision '%s' (%s) : %s", node.ID(), node.Hostname(), err)
		}

	} else if options.ProvisionTTL > 0 &&
		record.Status == Running && time.Since(time.Unix(record.Timestamp, 0)) > options.ProvisionTTL {
		log.Errorf("Provisioning of node '%s' has passed provisioning TTL of '%v'",
			node.Hostname(), options.ProvisionTTL)
		options.Provisioner.Clear(node.ID())
	} else {
		log.Debugf("Not invoking provisioning for '%s', current state is '%s'", node.Hostname(),
			record.Status.String())
	}

	return nil
}

// Done we are at the target state, nothing to do
var Done = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	// As devices are normally in the "COMPLETED" state we don't want to
	// log this fact unless we are in verbose mode. I suspect it would be
	// nice to log it once when the device transitions from a non COMPLETE
	// state to a complete state, but that would require keeping state.
	log.Debugf("COMPLETE: %s", node.Hostname())

	if options.AlwaysRename {
		updateNodeName(client, node, options)
	}

	return nil
}

// Deploy cause a node to deploy
var Deploy = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Infof("DEPLOY: %s", node.Hostname())

	if options.AlwaysRename {
		updateNodeName(client, node, options)
	}

	if !options.Preview {
		nodesObj := client.GetSubObject("nodes")
		myNode := nodesObj.GetSubObject(node.ID())
		// Start the node with the trusty distro. This should really be looked up or
		// a parameter default
		_, err := myNode.CallPost("start", url.Values{"distro_series": []string{"trusty"}})
		if err != nil {
			log.Errorf("DEPLOY '%s' : '%s'", node.Hostname(), err)
			return err
		}
	}
	return nil
}

// Aquire aquire a machine to a specific operator
var Aquire = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Infof("AQUIRE: %s", node.Hostname())
	nodesObj := client.GetSubObject("nodes")

	if options.AlwaysRename {
		updateNodeName(client, node, options)
	}

	if !options.Preview {
		// With a new version of MAAS we have to make sure the node is linked
		// to the subnet vid DHCP before we move to the Aquire state. To do this
		// We need to unlink the interface to the subnet and then relink it.
		//
		// Iterate through all the interfaces on the node, searching for ones
		// that are valid and not DHCP and move them to DHCP
		ifcsObj := client.GetSubObject("nodes").GetSubObject(node.ID()).GetSubObject("interfaces")
		ifcsListObj, err := ifcsObj.CallGet("", url.Values{})
		if err != nil {
			return err
		}

		ifcsArray, err := ifcsListObj.GetArray()
		if err != nil {
			return err
		}

		for _, ifc := range ifcsArray {
			ifcMap, err := ifc.GetMap()
			if err != nil {
				return err
			}

			// Iterate over the links assocated with the interface, looking for
			// links with a subnect as well as a mode of "auto"
			links, ok := ifcMap["links"]
			if ok {
				linkArray, err := links.GetArray()
				if err != nil {
					return err
				}

				for _, link := range linkArray {
					linkMap, err := link.GetMap()
					if err != nil {
						return err
					}
					subnet, ok := linkMap["subnet"]
					if ok {
						subnetMap, err := subnet.GetMap()
						if err != nil {
							return err
						}

						val, err := linkMap["mode"].GetString()
						if err != nil {
							return err
						}

						if val == "auto" {
							// Found one we like, so grab the subnet from the data and
							// then relink this as DHCP
							cidr, err := subnetMap["cidr"].GetString()
							if err != nil {
								return err
							}

							fifcID, err := ifcMap["id"].GetFloat64()
							if err != nil {
								return err
							}
							ifcID := strconv.Itoa(int(fifcID))

							flID, err := linkMap["id"].GetFloat64()
							if err != nil {
								return err
							}
							lID := strconv.Itoa(int(flID))

							ifcObj := ifcsObj.GetSubObject(ifcID)
							_, err = ifcObj.CallPost("unlink_subnet", url.Values{"id": []string{lID}})
							if err != nil {
								return err
							}
							_, err = ifcObj.CallPost("link_subnet", url.Values{"mode": []string{"DHCP"}, "subnet": []string{cidr}})
							if err != nil {
								return err
							}
						}
					}
				}
			}
		}
		_, err = nodesObj.CallPost("acquire",
			url.Values{"name": []string{node.Hostname()}})
		if err != nil {
			log.Errorf("AQUIRE '%s' : '%s'", node.Hostname(), err)
			return err
		}
	}
	return nil
}

// Commission cause a node to be commissioned
var Commission = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	updateNodeName(client, node, options)

	// Need to understand the power state of the node. We only want to move to "Commissioning" if the node
	// power is off. If the node power is not off, then turn it off.
	state := node.PowerState()
	switch state {
	case "on":
		// Attempt to turn the node off
		log.Infof("POWER DOWN: %s", node.Hostname())
		if !options.Preview {
			//POST /api/1.0/nodes/{system_id}/ op=stop
			nodesObj := client.GetSubObject("nodes")
			nodeObj := nodesObj.GetSubObject(node.ID())
			_, err := nodeObj.CallPost("stop", url.Values{"stop_mode": []string{"soft"}})
			if err != nil {
				log.Errorf("Commission '%s' : changing power start to off : '%s'", node.Hostname(), err)
			}
			return err
		}
		break
	case "off":
		// We are off so move to commissioning
		log.Infof("COMISSION: %s", node.Hostname())
		if !options.Preview {
			nodesObj := client.GetSubObject("nodes")
			nodeObj := nodesObj.GetSubObject(node.ID())

			updateNodeName(client, node, options)

			_, err := nodeObj.CallPost("commission", url.Values{})
			if err != nil {
				log.Errorf("Commission '%s' : '%s'", node.Hostname(), err)
			}
			return err
		}
		break
	default:
		// We are in a state from which we can't move forward.
		log.Warningf("%s has invalid power state '%s'", node.Hostname(), state)

		// If a power helper script is set, we have an unknown power state, and
		// we have not power type then attempt to use the helper script to discover
		// and set the power settings
		if options.PowerHelper != "" && node.PowerType() == "" {
			cmd := exec.Command(options.PowerHelper,
				append([]string{options.PowerHelperUser, options.PowerHelperHost},
					node.MACs()...)...)
			stdout, err := cmd.Output()
			if err != nil {
				log.Errorf("Failed while executing power helper script '%s' : %s",
					options.PowerHelper, err)
				return err
			}
			power := Power{}
			err = json.Unmarshal(stdout, &power)
			if err != nil {
				log.Errorf("Failed to parse output of power helper script '%s' : %s",
					options.PowerHelper, err)
				return err
			}
			switch power.Name {
			case "amt":
				params := map[string]string{
					"mac_address":   power.MacAddress,
					"power_pass":    power.PowerPassword,
					"power_address": power.PowerAddress,
				}
				node.UpdatePowerParameters(power.Name, params)
			default:
				log.Warningf("Unsupported power type discovered '%s'", power.Name)
			}
		}
		break
	}
	return nil
}

// Wait a do nothing state, while work is being done
var Wait = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Infof("WAIT: %s", node.Hostname())
	return nil
}

// Fail a state from which we cannot, currently, automatically recover
var Fail = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Infof("FAIL: %s", node.Hostname())
	return nil
}

// AdminState an administrative state from which we should make no automatic transition
var AdminState = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Infof("ADMIN: %s", node.Hostname())
	return nil
}

func findActions(target string, current string) ([]Action, error) {
	targets, ok := Transitions[target]
	if !ok {
		log.Warningf("unable to find transitions to target state '%s'", target)
		return nil, fmt.Errorf("Could not find transition to target state '%s'", target)
	}

	actions, ok := targets[current]
	if !ok {
		log.Warningf("unable to find transition from current state '%s' to target state '%s'",
			current, target)
		return nil, fmt.Errorf("Could not find transition from current state '%s' to target state '%s'",
			current, target)
	}

	return actions, nil
}

// ProcessActions
func ProcessActions(actions []Action, client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	var err error
	for _, action := range actions {
		if err = action(client, node, options); err != nil {
			log.Errorf("Error while processing action for node '%s' : %s",
				node.Hostname(), err)
			break
		}
	}
	return err
}

// ProcessNode something
func ProcessNode(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	substatus, err := node.GetInteger("substatus")
	if err != nil {
		return err
	}
	actions, err := findActions("Deployed", MaasNodeStatus(substatus).String())
	if err != nil {
		return err
	}

	if options.Preview {
		ProcessActions(actions, client, node, options)
	} else {
		go ProcessActions(actions, client, node, options)
	}
	return nil
}

func buildFilter(filter []string) ([]*regexp.Regexp, error) {

	results := make([]*regexp.Regexp, len(filter))
	for i, v := range filter {
		r, err := regexp.Compile(v)
		if err != nil {
			return nil, err
		}
		results[i] = r
	}
	return results, nil
}

func matchedFilter(include []*regexp.Regexp, target string) bool {
	for _, e := range include {
		if e.MatchString(target) {
			return true
		}
	}
	return false
}

// ProcessAll something
func ProcessAll(client *maas.MAASObject, nodes []MaasNode, options ProcessingOptions) []error {
	errors := make([]error, len(nodes))
	includeHosts, err := buildFilter(options.Filter.Hosts.Include)
	if err != nil {
		log.Fatalf("[error] invalid regular expression for include filter '%s' : %s", options.Filter.Hosts.Include, err)
	}

	includeZones, err := buildFilter(options.Filter.Zones.Include)
	if err != nil {
		log.Fatalf("[error] invalid regular expression for include filter '%v' : %s", options.Filter.Zones.Include, err)
	}

	for i, node := range nodes {
		// For hostnames we always match on an empty filter
		if len(includeHosts) >= 0 && matchedFilter(includeHosts, node.Hostname()) {

			// For zones we don't match on an empty filter
			if len(includeZones) >= 0 && matchedFilter(includeZones, node.Zone()) {
				err := ProcessNode(client, node, options)
				if err != nil {
					errors[i] = err
				} else {
					errors[i] = nil
				}
			} else {
				log.Debugf("ignoring node '%s' as its zone '%s' didn't match include zone name filter '%v'",
					node.Hostname(), node.Zone(), options.Filter.Zones.Include)
			}
		} else {
			log.Debugf("ignoring node '%s' as it didn't match include hostname filter '%v'",
				node.Hostname(), options.Filter.Hosts.Include)
		}
	}
	return errors
}
