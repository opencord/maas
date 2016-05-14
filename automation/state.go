package main

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"

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

// ProcessingOptions used to determine on what hosts to operate
type ProcessingOptions struct {
	Filter struct {
		Zones struct {
			Include []string
			Exclude []string
		}
		Hosts struct {
			Include []string
			Exclude []string
		}
	}
	Mappings     map[string]interface{}
	Verbose      bool
	Preview      bool
	AlwaysRename bool
}

// Transitions the actual map
//
// Currently this is a hand compiled / optimized "next step" table. This should
// really be generated from the state machine chart input. Once this has been
// accomplished you should be able to determine the action to take given your
// target state and your current state.
var Transitions = map[string]map[string]Action{
	"Deployed": {
		"New":                 Commission,
		"Deployed":            Done,
		"Ready":               Aquire,
		"Allocated":           Deploy,
		"Retired":             AdminState,
		"Reserved":            AdminState,
		"Releasing":           Wait,
		"DiskErasing":         Wait,
		"Deploying":           Wait,
		"Commissioning":       Wait,
		"Missing":             Fail,
		"FailedReleasing":     Fail,
		"FailedDiskErasing":   Fail,
		"FailedDeployment":    Fail,
		"Broken":              Fail,
		"FailedCommissioning": Fail,
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
        (Broken)->(Ready)`
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
		if entry, ok := options.Mappings[mac]; ok {
			if name, ok := entry.(map[string]interface{})["hostname"]; ok && current != name.(string) {
				nodesObj := client.GetSubObject("nodes")
				nodeObj := nodesObj.GetSubObject(node.ID())
				log.Printf("RENAME '%s' to '%s'\n", node.Hostname(), name.(string))

				if !options.Preview {
					nodeObj.Update(url.Values{"hostname": []string{name.(string)}})
				}
			}
		}
	}
	return nil
}

// Done we are at the target state, nothing to do
var Done = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	// As devices are normally in the "COMPLETED" state we don't want to
	// log this fact unless we are in verbose mode. I suspect it would be
	// nice to log it once when the device transitions from a non COMPLETE
	// state to a complete state, but that would require keeping state.
	if options.Verbose {
		log.Printf("COMPLETE: %s", node.Hostname())
	}

	if options.AlwaysRename {
		updateNodeName(client, node, options)
	}

	return nil
}

// Deploy cause a node to deploy
var Deploy = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Printf("DEPLOY: %s", node.Hostname())

	if options.AlwaysRename {
		updateNodeName(client, node, options)
	}

	if !options.Preview {
		nodesObj := client.GetSubObject("nodes")
		myNode := nodesObj.GetSubObject(node.ID())
		// Start the node with the trusty distro. This should really be looked up or
		// a parameter default
		_, err := myNode.CallPost("start", url.Values {"distro_series" : []string{"trusty"}})
		if err != nil {
			log.Printf("ERROR: DEPLOY '%s' : '%s'", node.Hostname(), err)
			return err
		}
	}
	return nil
}

// Aquire aquire a machine to a specific operator
var Aquire = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Printf("AQUIRE: %s", node.Hostname())
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
			log.Printf("ERROR: AQUIRE '%s' : '%s'", node.Hostname(), err)
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
		log.Printf("POWER DOWN: %s", node.Hostname())
		if !options.Preview {
                        //POST /api/1.0/nodes/{system_id}/ op=stop
			nodesObj := client.GetSubObject("nodes")
			nodeObj := nodesObj.GetSubObject(node.ID())
			_, err := nodeObj.CallPost("stop", url.Values{"stop_mode" : []string{"soft"}})
			if err != nil {
				log.Printf("ERROR: Commission '%s' : changing power start to off : '%s'", node.Hostname(), err)
			}
			return err
		}
		break
	case "off":
		// We are off so move to commissioning
		log.Printf("COMISSION: %s", node.Hostname())
		if !options.Preview {
			nodesObj := client.GetSubObject("nodes")
			nodeObj := nodesObj.GetSubObject(node.ID())

			updateNodeName(client, node, options)

			_, err := nodeObj.CallPost("commission", url.Values{})
			if err != nil {
				log.Printf("ERROR: Commission '%s' : '%s'", node.Hostname(), err)
			}
			return err
		}
		break
	default:
		// We are in a state from which we can't move forward.
		log.Printf("ERROR: %s has invalid power state '%s'", node.Hostname(), state)
		break
	}
	return nil
}

// Wait a do nothing state, while work is being done
var Wait = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Printf("WAIT: %s", node.Hostname())
	return nil
}

// Fail a state from which we cannot, currently, automatically recover
var Fail = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Printf("FAIL: %s", node.Hostname())
	return nil
}

// AdminState an administrative state from which we should make no automatic transition
var AdminState = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	log.Printf("ADMIN: %s", node.Hostname())
	return nil
}

func findAction(target string, current string) (Action, error) {
	targets, ok := Transitions[target]
	if !ok {
		log.Printf("[warn] unable to find transitions to target state '%s'", target)
		return nil, fmt.Errorf("Could not find transition to target state '%s'", target)
	}

	action, ok := targets[current]
	if !ok {
		log.Printf("[warn] unable to find transition from current state '%s' to target state '%s'",
			current, target)
		return nil, fmt.Errorf("Could not find transition from current state '%s' to target state '%s'",
			current, target)
	}

	return action, nil
}

// ProcessNode something
func ProcessNode(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	substatus, err := node.GetInteger("substatus")
	if err != nil {
		return err
	}
	action, err := findAction("Deployed", MaasNodeStatus(substatus).String())
	if err != nil {
		return err
	}

	if options.Preview {
		action(client, node, options)
	} else {
		go action(client, node, options)
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
				if options.Verbose {
					log.Printf("[info] ignoring node '%s' as its zone '%s' didn't match include zone name filter '%v'",
						node.Hostname(), node.Zone(), options.Filter.Zones.Include)
				}
			}
		} else {
			if options.Verbose {
				log.Printf("[info] ignoring node '%s' as it didn't match include hostname filter '%v'",
					node.Hostname(), options.Filter.Hosts.Include)
			}
		}
	}
	return errors
}
