#!/usr/bin/python

# Copyright 2017-present Open Networking Foundation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

DOCUMENTATION = '''
---
module: maas_cluster_interface
short_description: Manage MAAS Clusters Interfaces
options:
  maas:
    description:
      - URL of MAAS server
    default: http://localhost/MAAS/api/1.0/
  key:
    description:
      - MAAS API key
    required: yes
  cluster_name:
    description:
      - name of the cluster for the interface
    required: yes
  name:
    description:
      - name of the cluster interface
    required: yes
  management:
    description:
      - indicates how or if MAAS manages this interface
    choices: ['unmanaged', 'dhcp', 'dhcpdns']
    default: unmanaged
  interface:
    description:
      - the physical NIC for the interface
    required: no
  ip:
    description:
      - IP address assigned for this interface
    required: no
  subnet_mask:
    description:
      - network subnet mask for this interface
    required: no
  broadcast_ip:
    description:
      - broadcast IP for this interfaece's network
    required: no
  router_ip:
    description:
      - gateway router IP for this interface's network
    required: no
  ip_range_low:
    description:
      - the low range for dynamic IP address assignement
    required: no
  ip_range_high:
    description:
      - the high range for dynamic IP address assignment
    required: no
  static_ip_range_low:
    description:
      - the low range for static IP address assignment
    required: no
  static_ip_range_high:
    description:
      - the high range for static IP address assignment
    required: no
  state:
    description:
      - possible states for this cluster interface
    choices: ['present', 'absent', 'query']
    default: present

requirements: [ipaddress, requests_oauthlib, maasclient]
author: David Bainbridge
'''

EXAMPLES = '''
examples:
  maas_cluster_interface:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MyClusterInterface
    interface: eth0
    cluster_name: MyCluster
    ip: 172.16.10.2
    subnet_mask: 255.255.255.0
    broadcast_ip: 172.16.10.255
    router_ip: 172.16.10.1
    ip_range_low: 172.16.10.3
    ip_range_high: 172.16.10.127
    static_ip_range_low: 172.16.10.128
    static_ip_range_high: 172.16.10.253
    management: dhcpdns
    status: enabled
    state: present

  maas_cluster_interface:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MyDeadClusterInterface
    state: absent
'''

import sys
import json
import ipaddress
import requests
from maasclient.auth import MaasAuth
from maasclient import MaasClient

# For some reason the maasclient doesn't provide a put method. So
# we will add it here
def put(client, url, params=None):
    return requests.put(url=client.auth.api_url + url,
                        auth=client._oauth(), data=params)

# Attempt to interpret the given value as a JSON object, if that fails
# just return it as a string
def string_or_object(val):
    try:
        return json.loads(val)
    except:
        return val

# Return a copy of the given dictionary with any `null` valued entries
# removed
def remove_null(d_in):
    d = d_in.copy()
    to_remove = []
    for k in d.keys():
        if d[k] == None:
            to_remove.append(k)
    for k in to_remove:
        del d[k]
    return d

# Deterine if two dictionaries are different
def different(have, want, debug):
    have_keys = have.keys()
    for key in want.keys():
        if (key in have_keys and want[key] != have[key]) or key not in have_keys:
            diff = {"diff": key, "want": want[key]}
            if key in have_keys:
                diff['have'] = have[key]
            else:
                diff['have'] = False
            debug.append(diff)
            return True
    return False

# Get an cluster from MAAS using its name, if not found return None
def get_cluster(maas, name):
    res = maas.get('/nodegroups/', dict(op='list'))
    if res.ok:
        for ng in json.loads(res.text):
            if ng['cluster_name'] == name:
                return ng
    return None

# Get an cluster interface from MAAS using its name, if not found return None
def get_cluster_interface(maas, cluster, name):
    res = maas.get('/nodegroups/%s/interfaces/%s/' % (cluster['uuid'], name))
    if res.ok:
        return json.loads(res.text)
    return None

# Create an cluster interface based on the value given
def create_cluster_interface(maas, cluster, cluster_interface):
    merged = cluster_interface.copy()
    merged['op'] = 'new'
    res = maas.post('/nodegroups/%s/interfaces/' % cluster['uuid'], merged)
    if res.ok:
        return { 'error': False, 'status': get_cluster_interface(maas, cluster, merged['name']) }
    return { 'error': True, 'status': string_or_object(res.text) }

# Delete an cluster interface based on the name
def delete_cluster_interface(maas, cluster, name):
    res = maas.delete('/nodegroups/%s/interfaces/%s/' % (cluster['uuid'], name))
    if res.ok:
        return { 'error': False }
    return { 'error': True, 'status': string_or_object(res.text) }

def update_cluster_interface(maas, cluster, have, want):
    merged = have.copy()
    merged.update(want)
    res = put(maas, '/nodegroups/%s/interfaces/%s/' % (cluster['uuid'], merged['name']), merged)
    if res.ok:
        return { 'error': False, 'status': get_cluster_interface(maas, cluster, merged['name']) }
    return { 'error': True, 'status': string_or_object(res.text) }

def main():
    module = AnsibleModule(
        argument_spec = dict(
            maas=dict(default='http://localhost/MAAS/api/1.0/'),
            key=dict(required=True),
            base=dict(required=False),
            cluster_name=dict(required=True),
            name=dict(required=True),
            interface=dict(required=False),
            ip=dict(required=False),
            subnet_mask=dict(required=False),
            management=dict(default='unmanaged', choices=['unmanaged', 'dhcp', 'dhcpdns']),
            ip_range_low=dict(required=False),
            ip_range_high=dict(required=False),
            static_ip_range_low=dict(required=False),
            static_ip_range_high=dict(required=False),
            broadcast_ip=dict(required=False),
            router_ip=dict(required=False),
            state=dict(default='present', choices=['present', 'absent', 'query'])
        ),
        supports_check_mode = False
    )

    maas = module.params['maas']
    key = module.params['key']
    state = module.params['state']

    management_map = {
        'unmanaged': 0,
        'dhcp': 1,
        'dhcpdns': 2
    }

    # Construct a sparsely populate desired state
    desired = remove_null({
        'name': module.params['name'],
        'interface': module.params['interface'],
        'ip': module.params['ip'],
        'subnet_mask': module.params['subnet_mask'],
        'management': management_map[module.params['management']],
        'ip_range_low': module.params['ip_range_low'],
        'ip_range_high': module.params['ip_range_high'],
        'static_ip_range_low': module.params['static_ip_range_low'],
        'static_ip_range_high': module.params['static_ip_range_high'],
        'broadcast_ip': module.params['broadcast_ip'],
        'router_ip': module.params['router_ip'],
    })

    debug = []

    # Authenticate into MAAS
    auth = MaasAuth(maas, key)
    maas = MaasClient(auth)

    # Attempt to locate the cluster on which we will be working, error out if it can't be found
    cluster = get_cluster(maas, module.params['cluster_name'])
    if not cluster:
        module.fail_json(msg='Unable to find specified cluster "%s", cannot continue' % module.params['cluster_name'])
        return

    debug.append({"desired": desired})

    # Attempt to get the cluster interface from MAAS
    cluster_interface = get_cluster_interface(maas, cluster, desired['name'])

    debug.append({"found": cluster_interface})

    # Actions if the cluster interface does not currently exist
    if not cluster_interface:
        if state == 'query':
            # If this is a query, returne it is not found
            module.exit_json(changed=False, found=False)
        elif state == 'present':
            # If this should be present, then attempt to create it
            res = create_cluster_interface(maas, cluster, desired)
            if res['error']:
                module.fail_json(msg=res['status'])
            else:
                module.exit_json(changed=True, cluster_interface=res['status'], debug=debug)
        else:
            # If this should be absent, then we are done and in the desired state
            module.exit_json(changed=False)

        # Done with cluster interfaces does not exists actions
        return

    # Actions if the cluster interface does exist
    if state == 'query':
        # If this is a query, return the cluster interface
        module.exit_json(changed=False, found=True, cluster_interface=cluster_interface)
    elif state == 'present':
        # If we want this to exists check to see if this is different and
        # needs updated
        if different(cluster_interface, desired, debug):
            res = update_cluster_interface(maas, cluster, cluster_interface, desired)
            if res['error']:
                module.fail_json(msg=res['status'])
            else:
                module.exit_json(changed=True, cluster_interface=res['status'], debug=debug)
        else:
            # No differences, to nothing to change
            module.exit_json(changed=False, cluster_interface=cluster_interface)
    else:
        # If we don't want this cluster interface, then delete it
        res = delete_cluster_interface(maas, cluster, cluster_interface['name'])
        if res['error']:
            module.fail_json(msg=res['status'])
        else:
            module.exit_json(changed=True, cluster_interface=cluster_interface)

# this is magic, see lib/ansible/module_common.py
#<<INCLUDE_ANSIBLE_MODULE_COMMON>>
if __name__ == '__main__':
    main()
