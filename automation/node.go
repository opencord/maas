package main

import (
	"fmt"

	maas "github.com/juju/gomaasapi"
)

// MaasNodeStatus MAAS lifecycle status for nodes
type MaasNodeStatus int

// MAAS Node Statuses
const (
	Invalid             MaasNodeStatus = -1
	New                 MaasNodeStatus = 0
	Commissioning       MaasNodeStatus = 1
	FailedCommissioning MaasNodeStatus = 2
	Missing             MaasNodeStatus = 3
	Ready               MaasNodeStatus = 4
	Reserved            MaasNodeStatus = 5
	Deployed            MaasNodeStatus = 6
	Retired             MaasNodeStatus = 7
	Broken              MaasNodeStatus = 8
	Deploying           MaasNodeStatus = 9
	Allocated           MaasNodeStatus = 10
	FailedDeployment    MaasNodeStatus = 11
	Releasing           MaasNodeStatus = 12
	FailedReleasing     MaasNodeStatus = 13
	DiskErasing         MaasNodeStatus = 14
	FailedDiskErasing   MaasNodeStatus = 15
)

var names = []string{"New", "Commissioning", "FailedCommissioning", "Missing", "Ready", "Reserved",
	"Deployed", "Retired", "Broken", "Deploying", "Allocated", "FailedDeployment",
	"Releasing", "FailedReleasing", "DiskErasing", "FailedDiskErasing"}

func (v MaasNodeStatus) String() string {
	return names[v]
}

// FromString lookup the constant value for a given node state name
func FromString(name string) (MaasNodeStatus, error) {
	for i, v := range names {
		if v == name {
			return MaasNodeStatus(i), nil
		}
	}
	return -1, fmt.Errorf("Unknown MAAS node state name, '%s'", name)
}

// MaasNode convenience wrapper for an MAAS node on top of a generic MAAS object
type MaasNode struct {
	maas.MAASObject
}

// GetString get attribute value as string
func (n *MaasNode) GetString(key string) (string, error) {
	return n.GetMap()[key].GetString()
}

// GetFloat64 get attribute value as float64
func (n *MaasNode) GetFloat64(key string) (float64, error) {
	return n.GetMap()[key].GetFloat64()
}

// ID get the system id of the node
func (n *MaasNode) ID() string {
	id, _ := n.GetString("system_id")
	return id
}

func (n *MaasNode) PowerState() string {
	state, _ := n.GetString("power_state")
	return state
}

// Hostname get the hostname
func (n *MaasNode) Hostname() string {
	hn, _ := n.GetString("hostname")
	return hn
}

// MACs get the MAC Addresses
func (n *MaasNode) MACs() []string {
	macsObj, _ := n.GetMap()["macaddress_set"]
	macs, _ := macsObj.GetArray()
	if len(macs) == 0 {
		return []string{}
	}
	result := make([]string, len(macs))
	for i, mac := range macs {
		obj, _ := mac.GetMap()
		addr, _ := obj["mac_address"]
		s, _ := addr.GetString()
		result[i] = s
	}

	return result
}

// Zone get the zone
func (n *MaasNode) Zone() string {
	zone := n.GetMap()["zone"]
	attrs, _ := zone.GetMap()
	v, _ := attrs["name"].GetString()
	return v
}

// GetInteger get attribute value as integer
func (n *MaasNode) GetInteger(key string) (int, error) {
	v, err := n.GetMap()[key].GetFloat64()
	if err != nil {
		return 0, err
	}
	return int(v), nil
}
