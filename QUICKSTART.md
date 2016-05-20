# Quick Start
This guide is meant to enable the user to quickly exercise the capabilities provided by the artifacts of this
repository. There are three high level tasks that can be exercised:
   - Create development environment
   - Build / Tag / Publish Docker images that support bare metal provisioning
   - Deploy the bare metal provisioning capabilities to a virtual machine (head node) and PXE boot a compute node

**Prerequisite: Vagrant is installed and operationally.**
_Note: This quick start guide has only been tested against Vagrant + VirtualBox, specially on MacOS._

## Create Development Environment
The development environment is required for the other tasks in this repository. The other tasks could technically
be done outside this Vagrant based development environment, but it would be left to the user to ensure
connectivity and required tools are installed. It is far easier to leverage the Vagrant based environment.

### Create Development Machine
To create the development machine the following single Vagrant command can be used. This will create an Ubuntu
14.04 LTS based virtual machine and install some basic required packages, such as Docker, Docker Compose, and
Oracle Java 8.
```
vagrant up maasdev
```

### Connect to the Development Machine
To connect to the development machine the following vagrant command can be used.
```
vagrant ssh maasdev -- -L 8888:10.100.198.202:80
```

__Enter the complete command specified above, including the options `-- -L 8888:10.100.198.202:80`. These are used
for port forwarding in order to make the MAAS UI visible from you local host and will be explained further in the
section on Verifying MAAS.__

### Complete
Once you have created and connected to the development environment this task is complete. The `maas` repository
files can be found on the development machine under `/maasdev`. This directory is mounted from the host machine
so changes made to files in this directory will be reflected on the host machine and vice-versa.

## Build / Tag / Publish Docker Images
Bare metal provisioning leverages three (3) utilities built and packaged as Docker container images. These 
utilities are:

   - cord-maas-bootstrap - (directory: bootstrap) run at MAAS installation time to customize the MAAS instance
     via REST interfaces
   - cord-maas-automation - (directory: automation) run on the head node to automate PXE booted servers
     through the MAAS bare metal deployment work flow
   - cord-maas-dhcp-harvester - (directory: harvester) run on the head node to facilitate CORD / DHCP / DNS
     integration so that all hosts can be resolved via DNS

### Build

Each of the Docker images can be built using a command of the form `./gradlew build<Util>Image`, where `<Util>`
can be `Bootstrap`, `Automation`, or `Harvester`. Building is the process of creating a local Docker image
for each utility.

_NOTE: The first time you run `./gradlew` it will download from the Internet the `gradle` binary and install it
locally. This is a one time operation._

```
./gradlew buildBootstrapImage
./gradlew buildAutomationImage
./gradlew buildHarvester
```

Additionally, you can build all the images by issuing the following command:

```
./gradlew buildImages
```

### Tag

Each of the Docker images can be tagged using a command of the form `./gradlew tag<Util>Image`, where `<Util>`
can be `Bootstrap`, `Automation`, or `Harvester`. Tagging is the process of applying a local name and version to 
the utility Docker images.

_NOTE: The first time you run `./gradlew` it will download from the Internet the `gradle` binary and install it
locally. This is a one time operation._

```
./gradlew tagBootstrapImage
./gradlew tagAutomationImage
./gradlew tagHarvester
```

Additionally, you can tag all the images by issuing the following command:

```
./gradlew tagImages
```

### Publish

Each of the Docker images can be published using a command of the form `./gradlew publish<Util>Image`, where
`<Util>` can be `Bootstrap`, `Automation`, or `Harvester`. Publishing is the process of uploading the locally
named and tagged Docker image to a local Docker image registry.

_NOTE: The first time you run `./gradlew` it will download from the Internet the `gradle` binary and install it
locally. This is a one time operation._

```
./gradlew publishBootstrapImage
./gradlew publishAutomationImage
./gradlew publishHarvester
```

Additionally, you can publish all the images by issuing the following command:

```
./gradlew publishImages
```

### Complete
Once you have built, tagged, and published the utility Docker images this task is complete.

## Deploy Bare Metal Provisioning Capabilities
There are three parts to deploying bare metal: deploying the head node PXE server (`MAAS`), PXE
booting a compute node, and post deployment provisioning of the compute node. These tasks are accomplished
utilizing additionally Vagrant machines as well as executing `gradle` tasks in the Vagrant development machine.

### Create and Deploy MAAS into Head Node
The first task is to create the Vagrant base head node. This will create an additional Ubutu virtual
machine. **This task is executed on your host machine and not in the development virtual machine.** To create
the head node Vagrant machine issue the following command:

```
vagrant up headnode
```

### Deploy MAAS
Canonical MAAS provides the PXE and other bare metal provisioning services for CORD and will be deployed on the
head node via `Ansible`. To initiate this deployment issue the following `gradle` command. This `gradle` command
executes `ansible-playbook -i 10.100.198.202, --skip-tags=switch_support,interface_config --extra-vars=external_iface=eth0`.
The IP address, `10.100.198.202` is the IP address assigned to the head node on a private network. The
`skip-tags` option excludes Ansible tasks not required when utilizing the Vagrant based head node. The
`extra-vars` option overrides the default role vars for the external interface and is needed for the VirtualBox
based environment. Traffic from the compute nodes will be NAT-ed through this interface on the head node.

```
./gradlew deployMaas
```

This task can take some time so be patient. It should complete without errors, so if an error is encountered
something when horrible wrong (tm). 

### Verifying MAAS

After the Ansible script is complete the MAAS install can be validated by viewing the MAAS UI. When we
connected to the `maasdev` Vagrant machine the flags `-- -L 8888:10.100.198.202:80` were added to the end of
the `vagrant ssh` command. These flags inform Vagrant to expose port `80` on machine `10.100.198.202`
as port `8888` on your local machine. Essentially, expose the MAAS UI on port `8888` on your local machine.
To view the MAAS UI simply browser to `http://localhost:8888/MAAS`. 

You can login to MAAS using the user name `cord` and the password `cord`.

Browse around the UI and get familiar with MAAS via documentation at `http://maas.io`

** WAIT **

Before moving on MAAS need to download boot images for Ubuntu. These are the files required to PXE boot
additional servers. This can take several minutes. To view that status of this operation you can visit
the URL `http://localhost:8888/MAAS/images/`. When the downloading of images is complete it is possible to 
go to the next step.

### What Just Happened?

The proposed configuration for a CORD POD is has the following network configuration on the head node:

   - eth0 / eth1 - 40G interfaces, not relevant for the test environment.
   - eth2 - the interface on which the head node supports PXE boots and is an internally interface to which all
            the compute nodes connected
   - eth3 - WAN link. the head node will NAT from eth2 to eth3
   - mgmtbr - Not associated with a physical network and used to connect in the VM created by the openstack
              install that is part of XOS

The Ansible scripts configure MAAS to support DHCP/DNS/PXE on the eth2 and mgmtbr interfaces.

### Create and Boot Compute Node
To create a compute node you use the following vagrant command. This command will create a VM that PXE boots
to the interface on which the MAAS server is listening. **This task is executed on your host machine and not
in the development virtual machine.**
```
vagrant up computenode
```

Vagrant will create a UI, which will popup on your screen, so that the PXE boot process of the compute node can be
visually monitored. After an initial PXE boot of the compute node it will automatically be shutdown.

The compute node Vagrant machine it s bit different that most Vagrant machine because it is not created
with a user account to which Vagrant can connect, which is the normal behavior of Vagrant. Instead the
Vagrant files is configured with a _dummy_ `communicator` which will fail causing the following error
to be displayed, but the compute node Vagrant machine will still have been created correctly.
```
The requested communicator 'none' could not be found.
Please verify the name is correct and try again.
```

The compute node VM will boot, register with MAAS, and then be shut off. After this is complete an entry
for the node will be in the MAAS UI at `http://localhost:8888/MAAS/#/nodes`. It will be given a random
hostname made up, in the Canonical way, of a adjective and an noun, such as `popular-feast.cord.lab`. _The 
name will be different for everyone._ The new node will be in the `New` state.

For _real_ hosts, automation that leverages the MAAS APIs will transition the node from `New` through the
states of `Commissioning` and `Acquired` to `Deployed`.

#### Bad News
For hosts that support IPMI, which includes the hosts recommended for the CORD POD, MAAS will automatically
discover remote power management information. However, MAAS is not able to detect or manage power for 
VirtualBox machines.

A work-a-round has been created to support VirtualBox based VMs. This is accomplished by overriding the
support script for the `Intel AMT` power support module; but unfortunately it is not fully automated at this
point.

The work-a-round uses SSH from the MAAS head node to the machine that is running VirtualBox. To enable this,
assuming that VirtualBox is running on a Linux based system, you can copy the MAAS ssh public key from
`/var/lib/maas/.ssh/id_rsa.pub` on the head known to your accounts `authorized_keys` files. You can verify
that this is working by issuing the following commands from your host machine:
```
vagrant ssh headnode
sudo su - maas
ssh yourusername@host_ip_address
```

If you are able to accomplish these commands the VirtualBox power management should operate correctly.

To utilize this work-a-round the power management settings must be manual configured on the node. To 
accomplish this the VirtualBox ID for the compute node must first be discovered. To accomplish this issue
the following command on the machine on which VirtualBox is running:
```
vboxmanage list vms
```

This will return to a list of all VMs being managed by VirtualBox. In this list will be an entry similar
to the following for the compute node:
```
"maas_computenode_1463279637475_74274" {d18af821-91af-4d20-b3b2-67ed85e23c13}
```

The important part of this entry is the last 12 characters of the VM ID, `67ed85e23c13` in this example. This
will be used as a fake MAC address for power management.

To set the power settings for the compute node visit the MAAS UI page for the compute node. From there select
the `Power type` to `Intel AMT`. This will display the additional fields: `MAC Address`, `Power Password`, and
`Power Address`. The values of these fields should be set as follows:

   - MAC Address - the previously discovered last 12 characters of the VM ID, formatted like a MAC address,
                   `67:ed:85:e2:3c:13` from the example above.
   - Power Password - the user name to use to ssh from the head node to the host on which VirtualBox is 
                      executing.
   - Power Address - the IP address of the host on which VirtualBox is executing.

Once this information is saved the automation will eventually start the the compute node should transition
to `Deployed` state. This will include several reboots and shutdowns of the compute node.

### Post Deployment Provisioning of the Compute Node
Once the node is in the `Deployed` state, it can be provisioned for use in a CORD POD. Eventually, this action
will be automated and triggered when a node moves to the `Deployed` state. Currently, this must be manually 
invoked. The post deployment provisioning consists of configuring the networking on the node to comply with
best practices of a CORD POD installation and leverages an Ansible play boot to accomplish the provisioning.

To provision the compute node `ssh` to the head node,  change to the `/etc/maas/ansible` directory, and invoke
the playbook on the new compute node. To ssh to the head node, you can use `vagrant ssh headnode` from the 
host system for the virtual machines. Once __on__ the head node the following commands can invoke the
provisioning of the compute node, __the name of the compute node is an example name, the actual name in the
deployment can be found in the nodes list of the MAAS UI.__:

```
cd /etc/maas/ansible
ansible-playbook -i popular-feast.cord.lab, compute-node.yml --skip-tags=interface_config
```

_NOTE: the `skip-tags` option is required for the VirtualBox based environment_

### Complete
Once the compute node is in the `Deployed` state and post deployment provisioning on the compute node is
complete, this task is complete.
