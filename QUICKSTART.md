# Quick Start
This guide is meant to enable the user to quickly exercise the capabilities provided by the artifacts of this
repository. There are three high level tasks that can be exercised:
   - Create development environment
   - Build / Tag / Publish Docker images that support bare metal provisioning
   - Deploy the bare metal provisioning capabilities to a virtual machine (head node) and PXE boot a compute node

**Prerequisite: Vagrant is installed and operationally.**
_Note: This quick start guide has only been tested againt Vagrant + VirtualBox, specially on MacOS._

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

__Ignore the extra options at the end of this command after the `--`. These are used for port forwarding and
will be explaned later in the section on Verifing MAAS.__

### Complete
Once you have created and connected to the development environment this task is complete. The `maas` repository
files can be found on the development machine under `/maasdev`. This directory is mounted from the host machine
so changes made to files in this directory will be reflected on the host machine and vis vera.

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
There are two parts to deploying bare metal: deploying the head node PXE server (`MAAS`) and test PXE
booting a compute node. These tasks are accomplished utilizing additionally Vagrant machines as well
as executing `gradle` tasks in the Vagrant development machine.

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
exexcutes `ansible-playbook -i 10.100.198.202, --skip-tags=switch_support,interface_config`. The IP address,
`10.100.198.202` is the IP address assigned to the head node on a private network. The `skip-tags` option
excludes Ansible tasks not required when utilizing the Vagrant based head node.

```
./gradlew deployMaas
```

This task can take some time so be patient. It should complete without errors, so if an error is encountered
something when horrible wrong (tm). 

### Verifing MAAS

After the Ansible script is complete the MAAS install can be validated by viewing the MAAS UI. When we
connected to the `maasdev` Vagrant machine the flags `-- -L 8888:10.100.198.202:80` were added to the end of
the `vagrant ssh` command. These flags inform Vagrant to expose port `80` on machine `10.100.198.202`
as port `8888` on your local machine. Essentially, expose the MAAS UI on port `8888` on your local machine.
To view the MAAS UI simply browser to `http://localhost:8888/MAAS`. 

You can login to MAAS using the username `cord` and the password `cord`.

Browse around the UI and get familiar with MAAS via documentation at `http://maas.io`

## Create and Boot Compute Node
The requested communicator 'none' could not be found.
Please verify the name is correct and try again.
