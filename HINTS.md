# HINTS
This document contains hints and trouble shooting tips that might be helpful
when deploying a CORD POD. This tips are specific to the automation of the
deployment, which is the focus of this repository.

## Micro services
Automation of the CORD POD is driven by a set of micro services run in Docker
containers. Information about the configuration and REST API for those micro
services can be found in the [API Document](API.md).

## Useful Script
While is is possible to get the IP address of a container using the `docker
inspect` command, this can cause a lot of typing. The following script can
be useful to use to quickly determine the IP address of a container.

Save the script to a file such as `/usr/local/bin/docker-ip` and then you can
get the IP address of the container using `docker-ip <name>` or embeded it
in other commands such as curl by doing
`curl -sS http://$(docker-ip <name>):4243/provision/`

```
#!/bin/bash

test $# -ne 1 && echo "must specify the name of a container" && exit 1

IP=$(docker inspect --format '{{.NetworkSettings.IPAddress}}' $1)

if [ "$IP x" == " x" ]; then
  IP=$(docker inspect --format '{{.NetworkSettings.Networks.maas_default.IPAddress}}' $1)
fi

/bin/echo -ne $IP
```

## Viewing Provisioning Logs
The logs for the provisioning of a compute node or switch can be found on the
head node in `/etc/maas/ansible/logs`. The files are named with the provision
requests ID and the suffix `.log`.

These files can be useful when attempting to understand why provisioning may
be failing.

## Debugging Provisioning
The scripts used for provisioning and the ansible roles can be found in
`/etc/maas/ansible`. When debugging or understanding provisioning issues
it can be useful to edit these files so that further debug information will
be included in the provisioning log files.

## Force a Re-Provisioning of Switch or Compute Node
The provisioning state of nodes is managed via `Consul` which is a distributed
key value store. The backing for storage is the host file system and thus the
state is persisted over restarts.

Provisioning is configured by default to only perform a provisioning once. This
means after the initial provisioning is complete devices will not re-provision
unless a compute node is re-deployed or a switch is forced to provision.

The easiest way to force a re-provision is to delete the provisioning record
from the provision micro serice, then on the next cycle of automation either
the `automation` or `switchq` micro service will notice that no provisioning
record exists for the node and will re-invoke the provisioning.

To delete a provisioning record, first locate the ID of the record you wish
to delete. This can be done by quering all provisioning records and finding
the ID in the result.

1. Use `docker inspect` to discover the IP address of the provisioner
```
docker inspect --format '{{.NetworkSettings.Networks.maas_default.IPAddress}}'  provisioner
```

2. Query the list of provisioning records
```
curl -sS http://172.19.0.3:4243/provision/ | jq .
```

3. Delete the request
```
curl -sS -XDELETE http://172.18.0.3:4243/provosion/{id}
```

## Force DNS Information Harvesting
Periodically IP to host name mapping information is harvested from the DHCP
server and the DNS server is updated. It can be useful sometime, particularly
during the VM creation in the `deployPlatform` phase of CORD to force this
collection. To do so you can leverage the API to the IP harvester.

```
curl -sS -XPOST $(docker inspect --format '{{.NetworkSettings.Networks.maas_default.IPAddress}}'  harvester):8954/harvest
```

This call will update the file `/etc/bind/maas/dhcp_harvester.inc` and have
`bind` reload its configuration files.

## Restart Automation
To restart all of the automation containers you can use `docker-compose`. The
following commands will kill all the containers, pull any updated images from
the docker repository, and then restart the containers with the new images.
```
docker-compose -f /etc/maas/automation-compose.yml kill
docker-compose -f /etc/maas/automation-compose.yml rm --all -f
docker-compose -f /etc/maas/automation-compose.yml pull
docker-compose -f /etc/maas/automation-compose.yml up -d
```

You can operate on a specific contianer only by specifying the container
name at the end of the command, such as the following.
```
docker-compose -f /etc/maas/automation-compose.yml kill provisioner
```
## Restart harvester
The harvester micro service is controlled separately from the rest of the
automation micro services. It is controlled via the docker compose file
`/etc/maas/harvester-compose.yml`.

## Micro Service Logs
To view the logs for a given container you can either use docker directly
against the container name / ID, e.g.,
```
docker logs provisioner
```
against the entire collection of containers, e.g.,
```
docker-compose -f /etc/maas/automation-compose.yml logs
```
or againse a specific container in the collection, e.g.,
```
docker-compose -f /etc/maas/automation-compose.yml logs provisioner
```

This can be useful to look for errors in the logs.

## Pull New images
The following command can be used on the head node to pull all the latest
docker images from the local docker registry into the docker daemon. This
would be done after updating the docker images in the registry.
```
for i in $(docker images | grep "docker-registry:5000" | awk '{printf("%s:%s\n", $1, $2)}'); do docker pull $i; done`
```

After doing this any container that you want to use the new image needs to be
killed, removed, and restarted.
