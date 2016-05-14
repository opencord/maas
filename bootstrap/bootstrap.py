#!/usr/bin/python

from __future__ import print_function
import sys
import json
import ipaddress
import requests
from optparse import OptionParser
from maasclient.auth import MaasAuth
from maasclient import MaasClient

# For some reason the maasclient doesn't provide a put method. So
# we will add it here
def put(client, url, params=None):
    return requests.put(url=client.auth.api_url + url,
                            auth=client._oauth(),
                            data=params)

def add_or_update_node_group_interface(client, ng, gw, foundIfc, ifcName, subnet):
    ip = ipaddress.IPv4Network(unicode(subnet, 'utf-8'))
    hosts = list(ip.hosts())

    # if the caller specified the default gateway then honor that, else used the default
    gw = gw or str(hosts[0])

    ifc = {
        'ip_range_high': str(hosts[-1]),
        'ip_range_low': str(hosts[2]),
	'static_ip_range_high' : None,
	'static_ip_range_low' : None,
        'management': 2,
        'name': ifcName,
        #'router_ip' : gw,
        #'gateway_ip' : gw,
        'ip': str(hosts[0]),
        'subnet_mask': str(ip.netmask),
        'broadcast_ip': str(ip.broadcast_address),
        'interface': ifcName,
    }

    if foundIfc is not None:
        print("INFO: network for specified interface, '%s', already exists" % (ifcName))

	resp = client.get('/nodegroups/' + ng['uuid'] + '/interfaces/' + ifcName + '/', dict())
	if int(resp.status_code / 100) != 2:
	    print("ERROR: unable to read specified interface, '%s', '%d : %s'"
	          % (ifcName, resp.status_code, resp.text), file=sys.stderr)
	    sys.exit(1)

	# A bit of a hack here. Turns out MAAS won't return the router_ip / gateway_ip value
	# so we can't really tell if that value is set correctly. So we will compare the
	# values we can and use that as the "CHANGED" value, but always set all values.

	# Save the compare value
	same = ifc == json.loads(resp.text)

	# Add router_ip and gateway_ip to the desired state so that those will be set
	ifc['router_ip'] = gw
	ifc['gateway_ip'] = gw

        # If the network already exists, update it with the information we want
        resp = put(client, '/nodegroups/' + ng['uuid'] + '/interfaces/' + ifcName + '/', ifc)
        if int(resp.status_code / 100) != 2:
            print("ERROR: unable to update specified network, '%s', on specified interface '%s', '%d : %s'"
                   % (subnet, ifcName, resp.status_code, resp.text), file=sys.stderr)
	    sys.exit(1)

        if not same:
            print("CHANGED: updated network, '%s', for interface '%s'" % (subnet, ifcName))
	else:
            print("INFO: Network settings for interface '%s' unchanged" % ifcName)

    else:
        # Add the operation
        ifc['op'] = 'new'
	ifc['router_ip'] = gw
        ifc['gateway_ip'] = gw

        resp = client.post('/nodegroups/' + ng['uuid'] + '/interfaces/', ifc)
        if int(resp.status_code / 100) != 2:
            print("ERROR: unable to create specified network, '%s', on specified interface '%s', '%d : %s'"
                % (subnet, ifcName, resp.status_code, resp.text), file=sys.stderr)
	    sys.exit(1)
        else:
            print("CHANGED: created network, '%s', for interface '%s'" % (subnet, ifcName))

    # Add the first host to the subnet as the dns_server
    subnets = None
    resp = client.get('/subnets/', dict())
    if int(resp.status_code / 100) != 2:
        print("ERROR: unable to query subnets: '%d : %s'" % (resp.status_code, resp.text))
	sys.exit(1)
    else:
        subnets = json.loads(resp.text)

    id = None
    for sn in subnets:
        if sn['name'] == subnet:
            id = str(sn['id'])
            break

    if id == None:
        print("ERROR: unable to find subnet entry for network '%s'" % (subnet))
	sys.exit(1)

    resp = client.get('/subnets/' + id + '/')
    if int(resp.status_code / 100) != 2:
        print("ERROR: unable to query subnet '%s': '%d : %s'" % (subnet, resp.status_code, resp.text))
	sys.exit(1)

    data = json.loads(resp.text)

    found = False
    for ns in data['dns_servers']:
        if unicode(ns) == unicode(hosts[0]):
	    found = True

    if not found:
        resp = put(client, '/subnets/' + id + '/', dict(dns_servers=[hosts[0]]))
	if int(resp.status_code / 100) != 2:
            print("ERROR: unable to query subnet '%s': '%d : %s'" % (subnet, resp.status_code, resp.text))
	    sys.exit(1)
	else:
	    print("CHANGED: updated DNS server information")
    else:
        print("INFO: DNS already set correctly")
    

def main():
    parser = OptionParser()
    parser.add_option('-c', '--config', dest='config_file',
        help="specifies file from which configuration should be read", metavar='FILE')
    parser.add_option('-a', '--apikey', dest='apikey',
        help="specifies the API key to use when accessing MAAS")
    parser.add_option('-u', '--url', dest='url', default='http://localhost/MAAS/api/1.0',
        help="specifies the URL on which to contact MAAS")
    parser.add_option('-z', '--zone', dest='zone', default='administrative',
        help="specifies the zone to create for manually managed hosts")
    parser.add_option('-i', '--interface', dest='interface', default='eth0:1',
        help="the interface on which to set up DHCP for POD local hosts")
    parser.add_option('-n', '--network', dest='network', default='10.0.0.0/16',
        help="subnet to use for POD local DHCP")
    parser.add_option('-b', '--bridge', dest='bridge', default='mgmtbr',
        help="bridge to use for host local VM allocation")
    parser.add_option('-t', '--bridge-subnet', dest='bridge_subnet', default='172.18.0.0/16',
        help="subnet to assign from for bridged hosts")
    parser.add_option('-r', '--cluster', dest='cluster', default='Cluster master',
        help="name of cluster to user for POD / DHCP")
    parser.add_option('-s', '--sshkey', dest='sshkey', default=None,
        help="specifies public ssh key")
    parser.add_option('-d', '--domain', dest='domain', default='cord.lab',
        help="specifies the domain to configure in maas")
    parser.add_option('-g', '--gateway', dest='gw', default=None,
        help="specifies the gateway to configure for servers")
    (options, args) = parser.parse_args()

    if len(args) > 0:
        print("unknown command line arguments specified", file=sys.stderr)
        parser.print_help()
        sys.exit(1)

    # If a config file was specified then read the config from that
    config = {}
    if options.config_file != None:
        with open(options.config_file) as config_file:
            config = json.load(config_file)

    # Override the config with any command line options
    if options.apikey == None:
        print("must specify a  MAAS API key", file=sys.stderr)
        sys.exit(1)
    else:
        config['key'] = options.apikey
    if options.url != None:
        config['url'] = options.url
    if options.zone != None:
        config['zone'] = options.zone
    if options.interface != None:
        config['interface'] = options.interface
    if options.network != None:
        config['network'] = options.network
    if options.bridge != None:
        config['bridge'] = options.bridge
    if options.bridge_subnet != None:
        config['bridge-subnet'] = options.bridge_subnet
    if options.cluster != None:
        config['cluster'] = options.cluster
    if options.domain != None:
        config['domain'] = options.domain
    if options.gw != None:
        config['gw'] = options.gw
    if not 'gw' in config.keys():
        config['gw'] = None
    if options.sshkey == None:
        print("must specify a SSH key to use for cord user", file=sys.stderr)
        sys.exit(1)
    else:
        config['sshkey'] = options.sshkey
    
    auth = MaasAuth(config['url'], config['key'])
    client = MaasClient(auth)

    resp = client.get('/account/prefs/sshkeys/', dict(op='list'))
    if int(resp.status_code / 100) != 2:
        print("ERROR: unable to query SSH keys from server '%d : %s'"
	        % (resp.status_code, resp.text), file=sys.stderr)
	sys.exit(1)

    found_key = False
    keys = json.loads(resp.text)
    for key in keys:
	if key['key'] == config['sshkey']:
	    print("INFO: specified SSH key already exists")
            found_key = True

    # Add the SSH key to the user
    # POST /api/2.0/account/prefs/sshkeys/ op=new
    if not found_key:
        resp = client.post('/account/prefs/sshkeys/', dict(op='new', key=config['sshkey']))
        if int(resp.status_code / 100) != 2:
            print("ERROR: unable to add sshkey for user: '%d : %s'"
                    % (resp.status_code, resp.text), file=sys.stderr)
            sys.exit(1)
	else:
	    print("CHANGED: updated ssh key")
    
    # Check to see if an "administrative" zone exists and if not
    # create one
    found = None
    zones = client.zones
    for zone in zones:
        if zone['name'] == config['zone']:
            found=zone
    
    if found is not None:
        print("INFO: administrative zone, '%s', already exists" % config['zone'], file=sys.stderr)
    else:
        if not client.zone_new(config['zone'], "Zone for manually administrated nodes"):
            print("ERROR: unable to create administrative zone '%s'" % config['zone'], file=sys.stderr)
            sys.exit(1)
        else:
            print("CHANGED: Zone '%s' created" % config['zone'])
    
    # If the interface doesn't already exist in the cluster then
    # create it. Look for the "Cluster Master" node group, but
    # if it is not found used the first one in the list, if the
    # list is empty, error out
    found = None
    ngs = client.nodegroups
    for ng in ngs:
        if ng['cluster_name'] == config['cluster']:
            found = ng
            break
    
    if found is None:
        print("ERROR: unable to find cluster with specified name, '%s'" % config['cluster'], file=sys.stderr)
        sys.exit(1)

    resp = client.get('/nodegroups/' + ng['uuid'] + '/', dict())
    if int(resp.status_code / 100) != 2:
        print("ERROR: unable to get node group information for cluster '%s': '%d : %s'"
	    % (config['cluster'], resp.status_code, resp.text), file=sys.stderr)
	sys.exit(1)

    data = json.loads(resp.text)
    
    # Set the DNS domain name (zone) for the cluster
    if data['name'] != config['domain']:
        resp = put(client, '/nodegroups/' + ng['uuid'] + '/', dict(name=config['domain']))
        if int(resp.status_code / 100) != 2:
            print("ERROR: unable to set the DNS domain name for the cluster with specified name, '%s': '%d : %s'"
                % (config['cluster'], resp.status_code, resp.text), file=sys.stderr)
	    sys.exit(1)
        else:
            print("CHANGE: updated name of cluster to '%s' : %s" % (config['domain'], resp))
    else:
        print("INFO: domain name already set")
    
    found = None
    resp = client.get('/nodegroups/' + ng['uuid'] + '/interfaces/', dict(op='list'))
    if int(resp.status_code / 100) != 2:
        print("ERROR: unable to fetch interfaces for cluster with specified name, '%s': '%d : %s'"
            % (config['cluster'], resp.status_code, resp.text), file=sys.stderr)
        sys.exit(1)
    ifcs = json.loads(resp.text)

    localIfc = hostIfc = None 
    for ifc in ifcs:
        localIfc = ifc if ifc['name'] == config['interface'] else localIfc
        hostIfc = ifc if ifc['name'] == config['bridge'] else hostIfc

    add_or_update_node_group_interface(client, ng, config['gw'], localIfc, config['interface'], config['network'])
    add_or_update_node_group_interface(client, ng, config['gw'], hostIfc, config['bridge'], config['bridge-subnet'])

    # Update the server settings to upstream DNS request to Google
    # POST /api/2.0/maas/ op=set_config
    resp = client.get('/maas/', dict(op='get_config', name='upstream_dns'))
    if int(resp.status_code / 100) != 2:
        print("ERROR: unable to get the upstream DNS servers: '%d : %s'"
              % (resp.status_code, resp.text), file=sys.stderr)
	sys.exit(1)

    if unicode(json.loads(resp.text)) != u'8.8.8.8 8.8.8.4':
        resp = client.post('/maas/', dict(op='set_config', name='upstream_dns', value='8.8.8.8 8.8.8.4'))
        if int(resp.status_code / 100) != 2:
            print("ERROR: unable to set the upstream DNS servers: '%d : %s'"
                % (resp.status_code, resp.text), file=sys.stderr)
        else:
            print("CHANGED: updated up stream DNS servers")
    else:
        print("INFO: Upstream DNS servers correct")

    # Start the download of boot images
    resp = client.get('/boot-resources/', None)
    if int(resp.status_code / 100) != 2:
        print("ERROR: unable to read existing images download: '%d : %s'" % (resp.status_code, resp.text), file=sys.stderr)
	sys.exit(1)

    imgs = json.loads(resp.text)
    found = False
    for img in imgs:
	if img['name'] == u'ubuntu/trusty' and img['architecture'] == u'amd64/hwe-t':
	    found = True

    if not found:
        resp = client.post('/boot-resources/', dict(op='import'))
        if int(resp.status_code / 100) != 2:
            print("ERROR: unable to start image download: '%d : %s'" % (resp.status_code, resp.text), file=sys.stderr)
	    sys.exit(1)
        else:
            print("CHANGED: Image download started")
    else:
	print("INFO: required images already available")
    
if __name__ == '__main__':
    #try:
        main()
    #except:
#	e = sys.exc_info()[0]
#	print("ERROR: Unexpected exception: '%s'" % e, file=sys.stderr)
