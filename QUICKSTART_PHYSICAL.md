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

### Bootstrapping the Head Node from Outside the POD (OtP)
To deploy the head node it is assumed that the node is reachable from outside the POD over its *eth3* interface and that the machine from which you are
bootstrapping the head node has [`Vagrant`](https://www.vagrantup.com/) and [`git`](https://git-scm.com/) installed.

**NOTE:** *This quick start walk through assumes that the head node is being
deployed from the Vagrant machine that is defined within the repository. It is
possible to deployment the head node from the cloned repository without
using a Vagrant VM along as [`Ansible`](https://www.ansible.com/) version > 2.0 is installed on the OtP
host. When doing a deployment without the Vagrant VM, just invoke the given
Ansible commands directly from on the OtP host.*

#### Cloning the Repository
To clone the repository select a location on the outside the POD (OtP) host and
issue the `git` command to download (clone) the repository.
```
$ git clone http://gerrit.opencord.org/maas
```
When this is complete, a listing (`ls`) of this directory should yield output
similar to:
```
$ ls
QUICKSTART.md           automation/             doc/                    harvester/              roles/
QUICKSTART_PHYSICAL.md  bootstrap/              fabric.yml              head-node.yml           scripts/
README.md               build.gradle            gradle/                 head.yml
Vagrantfile             compute.yml             gradlew*                host_vars/
ansible/                dev-head-node.yml       gradlew.bat             hosts
```

#### Starting the Vagrant Machine
To start and connect to the the Vagrant machine, issue the following commands:
```
$ vagrant up maasdev
$ vagrant ssh maasdev
```
**NOTE:** *It may have several minutes for the first command `vagrant up maasdev` to complete as it will include creating the VM as well as downloading
and installing various software packages.*

Once connected to the Vagrant machine, you can find the deployment artifacts
in the `/maasdev` directory on the VM.
```
cd /maasdev
```

#### Invoke Bootstrapping
The head node will be bootstrapped using Ansible and the playbook
`head-node.yml`. This playbook is small and defines the Ansible role for the
head node:
```
- hosts: all
  serial: 1
  roles:
    - head-node
```
The `head-node` role depend on the Ansible `compute-node` role as well as
others, the important point being that a head node is simply a compute node
with some extra stuff installed.

To bootstrap the head node the following command can be issues:
```
$ ansible-playbook -i <ip-of-head-node>, --ask-pass --ask-sudo-pass \
  --user=<deployment-user-id> --extra-vars='fabric_ip=<fabric-ip> \
  management_ip=<management-ip> --exeternal_ip=<external-ip>' head-node.yml
```

##### Playbook Options
**NOTE** *The comma (,) after the <ip-of-head-node> is important as it
informs Ansible that the option is the IP of the head node and not an
inventory file that contains the list of IPs for the Ansible managed nodes.*

replace `<ip-of-head-node>` with the actually IP address to the host
on which the head node is being deployed and `<deployment-user-id>` with a
user ID on the host which can be used to `ssh` from the OtP host to the
head node and has `sudo` rights on the head node.

During the bootstrapping of the nodes various network settings are modified.
The `extra-vars` settings to the `ansible-playbook` command allow these
settings to be specified. The values for the `extra-vars` can be one of the
following:
- dhcp - assumes that the address will be assigned via DHCP
- manual - assumes that the address will be assigned manually
- a.b.c.d/# - specifies the IP address of the interface as well as the number
of bits in the netmask.

These values are used to configure the interfaces and will result in changes
to the `/etc/network/interface` file on the head node.

If you do not wish for the deployment scripts to modify the network
configuration of the head node you can substitute the `ansible-playbook` option `--skip-tags=interface_config` for the `extra-vars` options.

After you invoke the `ansible-playbook` you will be prompted for the `ssh` and
`sudo` passwords for the the remote user. In most cases these are the same.

The `ansible-playbook` will take several minutes to complete as it does
roughly the following:
1. Download and install Docker and Docker Compose
1. Rename and configured the network interfaces
1. Reboot the system to apply the network changes
1. Download boot images for the Accton switches
1. Download and install Canonical's Metal as a Service (MAAS) software
1. Configure MAAS
1. Download and invoke Docker images to support automation of MAAS
   capabilities

#### Wait for Image Download
As part of the bootstrapping and configuration of MAAS, downloading of boot
images for the other compute nodes is initiated. Before the other compute
nodes can be booted the download of this image must be completed.

To verify the status of the image download you can visit the MAAS UI at `http://<ip-of-head-node>/MAAS` and select the `Images` tab. On this page
the status of the download will be visible.

#### Complete
Once the download of boot image for the compute nodes is complete, the head
node is boot strapped and you can proceed to the section [Booting the Rest
of the POD]().

### Bootstrapping the Head Node from the Head Node
In order to bootstrap the head node from the head node Ansible, version >= 2,
must be installed on the head node. Additionally the following files /
directories from the repository must be on head node:
- `roles` (and all its sub-directories and files)
- `head-node.yml`

#### Invoke on the Head Node (OtH) Bootstrapping
Once Ansible is installed on the head node and the proper files are availble,
the head node can be bootstrapped. Because the bootstrapping capabilities
modify the network interface configuration of the head node, when
bootstrapping OtH must be done in two steps as there is a system reboot in
the middle. The first step is provisioning the head node as as a compute
node and the second is provisioning it with the head node capabilities.

### Bootstrap Head Node as Compute Node
To complete the first phase of the bootstrapping the head node, provisioning
as a compute node, the following command can be used:
```
$ ansible-playbook -i <ip-of-head-node>, --ask-pass --ask-sudo-pass \
  --user=<deployment-user-id> --extra-vars='fabric_ip=<fabric-ip> \
  management_ip=<management-ip> --exeternal_ip=<external-ip>' compute-node.yml
```

(see [Playbook Options](#playbook-options) for a description of the parameters)

If you do not wish to have the Ansible playbook to modify the network
configuration of the host you can add the `--skip-tags=interface_config`
option to the `ansible-playbook` command line.

If you do not wish the system to auto reboot if the network configuration is
modified you can add the `--skip-tags=reboot` option to the
`ansible-playbook` command line.

**NOTE:** *If the network parameters have changed the head node will likely
need to be rebooted for those changes to take effect. If you would like to
understand the changes before the reboot, you can specify the
`--skip-tags=reboot` option and then run the follow diff command:*
```
diff /etc/network/interfaces.1 /etc/network/interfaces
```

**NOTE:** *Be sure to reboot the head node after network changes so that they
will be applied.*

### Bootstrap Head Node as Head Node
Once the head node has been provisioned as a compute node the head node
capabilities can be overlaid. This can be done using the following command
line:
```
$ ansible-playbook -i <ip-of-head-node>, --ask-pass --ask-sudo-pass \
  --user=<deployment-user-id> --extra-vars='fabric_ip=<fabric-ip> \
  management_ip=<management-ip> --exeternal_ip=<external-ip>' --skip-tags=interface_config head-node.yml
```
(see [Playbook Options](#playbook-options) for a description of the parameters)

The `ansible-playbook` will take several minutes to complete as it does
roughly the following:
1. Download and install Docker and Docker Compose
1. Rename and configured the network interfaces
1. Reboot the system to apply the network changes
1. Download boot images for the Accton switches
1. Download and install Canonical's Metal as a Service (MAAS) software
1. Configure MAAS
1. Download and invoke Docker images to support automation of MAAS
   capabilities

#### Wait for Image Download
As part of the bootstrapping and configuration of MAAS, downloading of boot
images for the other compute nodes is initiated. Before the other compute
nodes can be booted the download of this image must be completed.

To verify the status of the image download you can visit the MAAS UI at `http://<ip-of-head-node>/MAAS` and select the `Images` tab. On this page
the status of the download will be visible.

#### Complete
Once the download of boot image for the compute nodes is complete, the head
node is boot strapped and you can proceed to the section [Booting the Rest
of the POD]().
