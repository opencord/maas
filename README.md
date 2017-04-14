# OpenCORD Bare Metal Provisioning

OpenCORD leverages Canonical's Metal as a Service (MAAS) solution. The MAAS solution provides a PXE boot environment. The basic
bare metal provisioning flow is:
   1. Install and provisioning MAAS and other utilities on one compute node that will have the **head node** role
   1. Boot the other components in a CORD POD (switches and other compute nodes)
   1. Once other components are operational perform some additional provisioning to prepare them to be part of a CORD POD and
      to be compliant with best practices of a CORD POD

After the base bare metal provisioning is complete further provisioning, such as XOS or leaf-spine fabric can be deployed.

# Ansible Roles Provided

## docker

Ensures Docker tools are available on the target system. Specifically `docker-engine` and `docker-compose`.

## java8-oracle

Ensures that the Oracle version of Java8 is available on the target system.

## fabric-switch

Ensures the OpenFlow agent (ofdpa) is available on the target switch as well as utility scripts that have been
helpful in the lab.

### Configuration

One of the scripts made available via this role is called `connect`. This script initiates the connection from
the switch to an OpenFlow controller. To connect to the controller the **DPID** of the switch is required as is
the **IP Address** of the SDN controller.

The **IP Address** of the controller is likely universal for all switches and can be either set via the command
line, using the `--extra-vars` command line option or via a global `vars` file.

The **DPID** is a per switch setting and represents the OpenFlow ID for the switch and will be in the for or
`0x0000000000000012` and will be unique for each switch. This value can be set either in a host specific variable
file or if the playbook is being run against a single switch using the `--extra-vars` command line option.

## compute-node

Ensures the 40G network interface card (NIC) drivers are installed and that the interfaces on the compute node are
named according to best practices. Specificall the 2 40G ports are `eth0` and `eth1`; the 2 10G ports are `eth2` and
`eth3`. Additionally, this roles sets a default password for the ubuntu user so that console logins are possible
for debug purposes. This last change, default password, should be eliminated for proxuction use.

### Configuration

Each compute node is statically assigned an IP address for the leaf-spine fabric. This can be configured
in a host specific variable file found in the `host_vars` directory or could be specified on the command line
using the `--extra-vars` option if the play book is be run against a single target system.

### Dependencies

This role depends on the `docker` role.

## onos-fabric

Ensures

## mmas

Ensures that Canonical's Metal as a Service (MAAS) is available on the target system and configured according to
best practices for a CORD POD. This role is meant to be applied to a head node in the CORD POD.

### Assumptions

   - A 2 port _Intel_ 40G card is installed on the head node
   - A 2 port 10G card is installed on the head node
   - Head Node has _Internet_ connectivity via the 10G interface named `eth3`

   _Additionally configuration variables, including network IP addressing information can be found in
   the file `vars/main.yml`._

### Configuration

    - When working with virtual machines, or really any machine that does not have a real 40G interface it is
      sometimes useful to not attempt to rename / reconfigure the machines interfaces. This can be accomplished
      by adding the `--skip-tags=interface_config` command line option to your ansible-playbook command line.

### Comments

   - `iptables` rules will be established to `NAT` traffic out interface `eth3`
   - This role installs and starts two docker images to help manage the MAAs install including:
      - `cord/maas-dhcp-harvester:0.1-prerelease` - adds DHCP addresses to the DNS server that MAAS misses
      - `cord/maas-automation:0.1-prerelease` - automates compute nodes through the states of MAAS to the
         deployed state so that they can use used as part of the CORD POD.

### Dependencies

This role depends on the `compute-node` role.

