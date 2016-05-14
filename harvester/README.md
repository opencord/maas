# DHCP/DNS Name and IP Harvester
This Python application and Docker image provide an utility that periodically parses the DHCP leases files and updates the `bind9` DNS configuration so that hosts
that are assigned IP addresses dynamically from DHCP can be looked up via DNS.

### Integration
There are several keys to making all this work. The utility needs to be able to read the DHCP lease file as well as write a file to a location that can be read
by the DNS server; so more than likely this utility should be run on the same host that is running DHCP and DNS. Additionally, this utility needs to be able to
run the bind9 utility `rndc` to reload the DNS zone. This means that it needs a `DNSSEC` key and secret to access the DNS server.

Lastly, this utility generates a file that can be `$include`-ed into a bind9 zone file, so the original zone file needs to be augmented with a `$INCLUDE` statement
that includes the files to which the uility is configured to write via the `-dest` command line option.

### Docker Build
To build the docker image use the command:
```
docker build -t harvester .
```

### Docker Run
To run the utility, a docker command similar to what is below may be used

```
docker run -d --name=dhcpharvester  \
    -v `pwd`/key:/key -v /var/lib/maas/dhcp:/dhcp -v /etc/bind/maas:/bind harvester \
    -f '^(?!cord)' -u -s 192.168.42.231 -p 954 -k /key/mykey.conf -z cord.lab  -r 5m \
    -y -t 1s
```

### API
There is a simple REST API on this utility so that an external client can asynchronously invoke the DHCP harvest behavior. The REST API is 
synchronous in that the request will not return a response until the harvest is complete. To invoke the request a `HTTP PUT` request needs
be sent to the utility, such as by curl:
```
curl -XPOST http://<apiserver>:<apiport>/harvest
```
Currently there is not security around this so it could be abused. There is some protection so that if the system is sent multple request
if won't actually reharvest until a quiet period has expired. The purpose is to not allow the system to be overloaded.

### Implementation Details
Internally the implementation uses threads and queues to communicate between the threads when the utility is in the mode to periodically 
harvest.

For the verification of IP addresses, i.e. pinging the hosts, worker threads are used to support concurrency, thus making the verification
process faster.
