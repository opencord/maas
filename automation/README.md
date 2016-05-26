# Metal as a Service Automation (maas-flow)
This is a utility that works in conjunction with an Ubuntu Metal as a Service
([MAAS](http://maas.io)) deployment. By default, the MAAS system allows an
operator to manually control the lifecycle of a compute host as it comes on
line leveraging PXE, DHCP, DNS, etc.

The utility leverages the MAAS REST API to periodically monitor the **status**
of the hosts under control of MAAS and continuous attempts to move those hosts
into a **deployed** state. (Note: this will likely change in the future to
support additional target states.)

### Filtering Hosts on which to Operate
Using a filter the operator can control on which hosts automation acts. The
filter is a basic **JSON** object and can either be specified as a string on
the command line or a file which contains the filter. When specifying a file
the value of the **-filter** command line option should be a **@** followed by
the name of the file, i.e. @$HOME/some/file, and it may container environment
variable.

The structure of the filter object is:
```
{
    "hosts" : {
        "include" : [],
        "exclude" : []
    },
    "zones" : {
        "include" : [],
        "exclude" : []
    }
}
```
For **hosts** the **include** and **exclude** values are a list of regular
expressions which are mapped against the hostname of a device under control of
MAAS.

for **zones** the **include** and **exclude** values are a list of regular
expression which are mapped against the zone with which a host is associated.

When both **include** and **exclude** values are specified the **include**
is processed followed by the **exclude**.

The default filter, if none is specified, is depicted below. Essentially it
specifies that the automation will act on all hosts in only the **default**
zone. (*NOTE: This default filter may change in the future.*)
```
{
  "hosts" : {
    "include" : [],
    "exclude" : []
  },
  "zones" : {
    "include" : ["default"],
    "exclude" : []
  }
}
```

*NOTE:* only include is currently (January 26, 2016) supported.

### Connecting to MAAS
The connection to MAAS is controlled by command line parameters, specifically:
* **-apiVersion** - (default: *1.0*) specifies the version of the MAAS API to use
* **-apiKey** - (default: *none*) specifies the API key to use to authenticate to
the MAAS server. For a given user this can be found on under their account
settings in the MAAS UI. This value is important as the automation is acting
on behalf of this user and the SSH keys that are pushed to hosts will be the
SSH keys associated with this user.
* **-maas** - (default: *http://localhost/MAAS*) specifies the base URL on which
to contact the MAAS server.
* **-period** - (default: *15s*) specifies how often the automation queries the
MAAS server to retrieve the state of the hosts. Automation must query the state
of the hosts from MAAS as MAAS does not support an asynchronous change
mechanism today. This value should be set such that the automation can fully
process all the hosts within a period.

### Docker Image
The project contains a `Dockerfile` that can be used to construct a docker
image from the repository. The docker image is also provided via Docker Hub at
https://hub.docker.com/r/ciena/maas-flow/.

### State machine
The state machine on which the MAAS automation is based is depicted below.
Currently (January 26, 2016) the automation only supports a deployed target
state and will not act on hosts that are in a failed, broken, or error state.
![](lifecycle.png)

### Post Deployment Provisioning
All the states in the state machine are defined and maintained by
MAAS except the states Provisioning, ProvisionError, and Provisioned. These
states are used to track the post-deployment provisioning that is part of the
automation.

Post deployment provisioning can be accomplished either by the specification of
a script to execute or the specification of a URL to trigger.

#### Executing a Script
A script to execute to post deploy provision a node can be specified via the
environment as `PROVISION_URL`. After a node is deployed this script will be
executed with three (3) parameters:
- node ID - the node ID that MAAS uses to track the node
- name - the name of the node in MAAS
- ip - the IP address assigned to the node

It is important to note that when executing a script that the script is run
within the docker container that is running the automation. Thus any script
must be mounted as a volume into the container.

#### Triggering a URL
Alternatively the automation can trigger a URL to POST deploy provision a node.
In this instance, automation will `POST` a `JSON` object to the specified URL
with the values:
- "id" - the node ID that MAAS uses to track the node
- "name" - the name of the node in MAAS
- "ip" - the IP address assigned to the node

The provider specified should return "202 Accept" to acknowledge the acceptance
of the request. The automation controller will poll for status on the
provisioning so the provider should accept the request but not process it
while the client is blocked.

The automation controller will periodically poll for provisioning status for a
given node by doing a `HTTP GET` on the specified provisioning URL appended with
`/` and the `ID` of the node. The provider should either return `202 Accept` if
the node is still being provisioned, `200 OK` if the provisioning is complete
and successful, or any other response which will be treated as an error.
