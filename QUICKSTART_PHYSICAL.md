# Quick Start for Physical CORD POD
This guide is meant to enable the user to utilize the artifacts of this
repository to to deploy CORD on to a physical hardware rack. The artifacts in
this repository will deploy CORD against a standard physical rack wired
according to the **best practices** as defined in this document.

## Physical configuration
![Physical Hardware Connectivity](doc/images/physical.png)

As depicted in the diagram above the base model for the CORD POD deployment
contains:
- 4 OF switches comprising the leaf - spine fabric utilized for data traffic
- 4 compute nodes with with 2 40G ports and 2 1G ports
- 1 top of rack (TOR) switch utilized for management communications

The best practices in terms of connecting the components of the CORD POD
include:
- Leaf nodes are connected to the spines nodes starting at the highest port
number on the leaf.
- For a given leaf node, its connection to the spine nodes terminate on the
same port number on each spine.
- Leaf *n* connections to spine nodes terminate at port *n* on each spine
node.
- Leaf spine switches are connected into the management TOR starting from the
highest port number.
- Compute nodes 40G interfaces are named *eth0* and *eth1*.
- Compute nodes 10G interfaces are named *eth2* and *eth3*.
- Compute node *n* is connected to the management TOR switch on port *n*,
egressing from the compute node at *eth2*.
- Compute node *n* is connected to its primary leaf, egressing at *eth0* and terminating on the leaf at port *n*.
- Compute node *n* is connected to its secondary leaf, egressing at *eth1* and
terminating on the leaf at port *n*.
- *eth3* on the head node is the uplink from the POD to the Internet.

The following assumptions are made about the phyical CORD POD being deployed:
- The leaf - spine switchs are Accton 6712s
- The compute nodes are using 40G Intel NIC cards
- The compute node that is to be designated the *head node* has
Ubuntu 14.04 LTS installed.

## Bootstrapping the Head Node
The head node is the key to the physical deployment of a CORD POD. The
automated deployment of the physical POD is designed such that the head node is
manually deployed, with the aid of automation tools, such as Ansible and from
this head node the rest of the POD deployment is automated.

The head node can be deployed either from a node outside the CORD POD or by
deploying from the head the head node. The procedure in each scenario is
slightly different because during the bootstrapping of the head node it is
possible that the interfaces needed to be renamed and the system to be
rebooted.

### Bootstrapping the Head Node from Outside the POD
To deploy the head node it is assumed that the node is reachable from outside
