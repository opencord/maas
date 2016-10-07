# DHCP/DNS Name and IP Harvester
This µservice and Docker image provide an utility that periodically parses the
DHCP leases database file and updates the `bind9` DNS configuration so that
hosts that are assigned IP addresses dynamically from DHCP can be looked up via
DNS.

### Integration
There are several keys to making all this work. The utility needs to be able to
read the DHCP lease database file as well as write a file to a location that
can be read by the DNS server; so more than likely this utility should be run
on the same host that is running DHCP and DNS. Additionally, this utility needs
to be able to run the bind9 utility `rndc` to reload the DNS zone. This means
that it needs a `DNSSEC` key and secret to access the DNS server.

Lastly, this utility generates a file that can be `$include`-ed into a bind9
zone file, so the original zone file needs to be augmented with a `$INCLUDE`
statement that includes the files to which the uility is configured to write.

### Docker Build
To build the docker image use the command:
```
docker build -t harvester .
```

### configuration
The utility is configured the environment following the 12 factor application
principles. The available configuration settings are:

| OPTION | DEFAULT | DESCRIPTION |
| --- | --- | --- |
| PORT | 4246 | port on which the service will listen for requests |
| LISTEN | 0.0.0.0 | IP on which the service will listen for requests |
| LOG_LEVEL | warning | log output level |
| LOG_FORMAT | text | format of log messages |
| DHCP_LEASE_FILE | /harvester/dhcpd.leases | lease file to parse for lease information |
| OUTPUT_FILE | | name of file to output discovered lease in bind9 format |
| OUTPUT_FORMAT | {{.ClientHostname}}\tIN A {{.IPAddress}}\t; {{.HardwareAddress}} | specifies the single entry format when outputing to a file |
| VERIFY_LEASES | true | verifies leases with a ping |
| VERIFY_TIMEOUT | 1s | max timeout (RTT) to wait for verification pings |
| VERIFY_WITH_UDP | false | use UDP instead of raw sockets for ping verification |
| QUERY_PERIOD | 30s | period at which the DHCP lease file is processed |
| QUIET_PERIOD | 2s | period to wait between accepting parse requests |
| REQUEST_TIMEOUT | 10s | period to wait for processing when requesting a DHCP lease database parsing |
| RNDC_UPDATE | false | determines if the harvester reloads the DNS servers after harvest |
| RNDC_ADDRESS | 127.0.0.1 | IP address of the DNS server to contact via RNDC |
| RNDC_PORT | 954 | port of the DNS server to contact via RNDC |
| RNDC_KEY_FILE | /key/rndc.conf.maas | key file, with default, to contact DNS server |
| RNDC_ZONE | cord.lab | zone to reload |

When configuring the µservice via the environment the name of the option should
be prefixed with `HARVESTER_`.

### Docker Run
To run the utility, a docker command similar to what is below may be used

```
docker run -d --name=harvester
```

### REST API

| RESOURCE | METHOD | DESCRIPTION |
| --- | --- | --- |
| /harvest | POST | Requests the processing of the DHCP lease database |
| /lease | GET | Returns the list of DHCP leases harvested from the DHCP server |
| /lease/{ip} | GET | Returns a single DHCP lease associated with the given IP |
| /lease/hostname/{name} | GET | Returns a single DHCP lease associated with the given client hostname |
| /lease/hardware/{mac} | GET | Returns a single DHCP lease associated with the given hardware addreaa |

Currently there is no security around this so it could be abused. There is some
protection so that if the system is sent multiple requests it won't actually
re-harvest until a quiet period has expired. The purpose is to not allow the
system to be overloaded.

### Implementation Details
Verification of leases is performed by doing an ICMP ping to the host.
