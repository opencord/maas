package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	ProvTracker  Tracker
	ProvisionURL string
	ProvisionTTL time.Duration
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
	(Provisioning)->(ProvisionError)
	(ProvisionError)->(Provisioning)
	(Provisioning)->(Provisioned)`
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

// Reset we are at the target state, nothing to do
var Reset = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	if options.Verbose {
		log.Printf("RESET: %s", node.Hostname())
	}

	if options.AlwaysRename {
		updateNodeName(client, node, options)
	}

	options.ProvTracker.Clear(node.ID())

	return nil
}

// Provision we are at the target state, nothing to do
var Provision = func(client *maas.MAASObject, node MaasNode, options ProcessingOptions) error {
	if options.Verbose {
		log.Printf("CHECK PROVISION: %s", node.Hostname())
	}

	if options.AlwaysRename {
		updateNodeName(client, node, options)
	}

	record, err := options.ProvTracker.Get(node.ID())
	if options.Verbose {
		log.Printf("[info] Current state of node '%s' is '%s'", node.Hostname(), record.State.String())
	}
	if err != nil {
		log.Printf("[warn] unable to retrieve provisioning state of node '%s' : %s", node.Hostname(), err)
	} else if record.State == Unprovisioned || record.State == ProvisionError {
		var err error = nil
		var callout *url.URL
		log.Printf("PROVISION '%s'", node.Hostname())
		if len(options.ProvisionURL) > 0 {
			if options.Verbose {
				log.Printf("[info] Provisioning callout to '%s'", options.ProvisionURL)
			}
			callout, err = url.Parse(options.ProvisionURL)
			if err != nil {
				log.Printf("[error] Failed to parse provisioning URL '%s' : %s", options.ProvisionURL, err)
			} else {
				ips := node.IPs()
				ip := ""
				if len(ips) > 0 {
					ip = ips[0]
				}
				switch callout.Scheme {
				// If the scheme is a file, then we will execute the refereced file
				case "", "file":
					if options.Verbose {
						log.Printf("[info] executing local script file '%s'", callout.Path)
					}
					record.State = Provisioning
					record.Timestamp = time.Now().Unix()
					options.ProvTracker.Set(node.ID(), record)
					err = exec.Command(callout.Path, node.ID(), node.Hostname(), ip).Run()
					if err != nil {
						log.Printf("[error] Failed to execute '%s' : %s", options.ProvisionURL, err)
					} else {
						if options.Verbose {
							log.Printf("[info] Marking node '%s' with ID '%s' as provisioned",
								node.Hostname(), node.ID())
						}
						record.State = Provisioned
						options.ProvTracker.Set(node.ID(), record)
					}

				default:
					if options.Verbose {
						log.Printf("[info] POSTing to '%s'", options.ProvisionURL)
					}
					data := map[string]string{
						"id":   node.ID(),
						"name": node.Hostname(),
						"ip":   ip,
					}
					hc := http.Client{}
					var b []byte
					b, err = json.Marshal(data)
					if err != nil {
						log.Printf("[error] Unable to marshal node data : %s", err)
					} else {
						var req *http.Request
						var resp *http.Response
						if options.Verbose {
							log.Printf("[debug] POSTing data '%s'", string(b))
						}
						req, err = http.NewRequest("POST", options.ProvisionURL, bytes.NewReader(b))
						if err != nil {
							log.Printf("[error] Unable to construct POST request to provisioner : %s",
								err)
						} else {
							req.Header.Add("Content-Type", "application/json")
							resp, err = hc.Do(req)
							if err != nil {
								log.Printf("[error] Unable to process POST request : %s",
									err)
							} else {
								defer resp.Body.Close()
								if resp.StatusCode == 202 {
									record.State = Provisioning
								} else {
									record.State = ProvisionError
								}
								options.ProvTracker.Set(node.ID(), record)
							}
						}
					}
				}
			}
		}

		if err != nil {
			if options.Verbose {
				log.Printf("[warn] Not marking node '%s' with ID '%s' as provisioned, because of error '%s'",
					node.Hostname(), node.ID(), err)
				record.State = ProvisionError
				options.ProvTracker.Set(node.ID(), record)
			}
		}
	} else if record.State == Provisioning && time.Since(time.Unix(record.Timestamp, 0)) > options.ProvisionTTL {
		log.Printf("[error] Provisioning of node '%s' has passed provisioning TTL of '%v'",
			node.Hostname(), options.ProvisionTTL)
		record.State = ProvisionError
		options.ProvTracker.Set(node.ID(), record)
	} else if record.State == Provisioning {
		callout, err := url.Parse(options.ProvisionURL)
		if err != nil {
			log.Printf("[error] Unable to parse provisioning URL '%s' : %s", options.ProvisionURL, err)
		} else if callout.Scheme != "file" {
			var req *http.Request
			var resp *http.Response
			if options.Verbose {
				log.Printf("[info] Fetching provisioning state for node '%s'", node.Hostname())
			}
			req, err = http.NewRequest("GET", options.ProvisionURL+"/"+node.ID(), nil)
			if err != nil {
				log.Printf("[error] Unable to construct GET request to provisioner : %s", err)
			} else {
				hc := http.Client{}
				resp, err = hc.Do(req)
				if err != nil {
					log.Printf("[error] Failed to quest provision state for node '%s' : %s",
						node.Hostname(), err)
				} else {
					switch resp.StatusCode {
					case 200: // OK - provisioning completed
						if options.Verbose {
							log.Printf("[info] Marking node '%s' with ID '%s' as provisioned",
								node.Hostname(), node.ID())
						}
						record.State = Provisioned
						options.ProvTracker.Set(node.ID(), record)
					case 202: // Accepted - in the provisioning state
						// Noop, presumably alread in this state
					default: // Consider anything else an erorr
						record.State = ProvisionError
						options.ProvTracker.Set(node.ID(), record)
					}
				}
			}
		}
	} else if options.Verbose {
		log.Printf("[info] Not invoking provisioning for '%s', currned state is '%s'", node.Hostname(),
			record.State.String())
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
		_, err := myNode.CallPost("start", url.Values{"distro_series": []string{"trusty"}})
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
			_, err := nodeObj.CallPost("stop", url.Values{"stop_mode": []string{"soft"}})
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

func findActions(target string, current string) ([]Action, error) {
	targets, ok := Transitions[target]
	if !ok {
		log.Printf("[warn] unable to find transitions to target state '%s'", target)
		return nil, fmt.Errorf("Could not find transition to target state '%s'", target)
	}

	actions, ok := targets[current]
	if !ok {
		log.Printf("[warn] unable to find transition from current state '%s' to target state '%s'",
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
			log.Printf("[error] Error while processing action for node '%s' : %s", node.Hostname, err)
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
