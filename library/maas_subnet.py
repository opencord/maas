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
module: maas_subnet
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
  name:
    description:
      - name of the subnet
    required: yes
  space:
    description:
      - network space of the subnet
  dns_servers:
    description:
      - dns servers for the subnet
  gateway_ip:
    description:
      - gateway IP for the subnet
  cidr:
    description:
      - cidr for the subnet
  state:
    description:
      - possible states for this subnet
    choices: ['present', 'absent', 'query']
    default: present

requirements: [ipaddress, requests_oauthlib, maasclient]
author: David Bainbridge
'''

EXAMPLES = '''
examples:
  maas_subnet:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MySubnet
    state: present

  maas_subnet:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MyDeadSubnet
    state: absent
'''

import sys
import json
import ipaddress
import requests
import string
from maasclient.auth import MaasAuth
from maasclient import MaasClient

debug = []

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

# Removes keys from a dictionary either using an include or
# exclude filter This change happens on given dictionary is
# modified.
def filter(filter_type, d, keys):
    if filter_type == 'include':
        for k in d.keys():
            if k not in keys:
                d.pop(k, None)
    else:
        for k in d.keys():
            if k in keys:
                d.pop(k, None)

# Converts a subnet structure with names for the vlan and space to their
# ID equivalents that can be used in a REST call to MAAS
def convert(maas, subnet):
    copy = subnet.copy()
    copy['space'] = get_space(maas, subnet['space'])['id']
    fabric_name, vlan_name = string.split(subnet['vlan'], ':', 1)
    fabric = get_fabric(maas, fabric_name)
    copy['vlan'] = get_vlan(maas, fabric, vlan_name)['id']
    return copy

# replaces the expanded VLAN object with a unique identifier of
# `fabric`:`name`
def simplify(subnet):
    copy = subnet.copy()
    if 'dns_servers' in copy.keys() and type(copy['dns_servers']) == list:
        copy['dns_servers'] = ",".join(copy['dns_servers'])
    if subnet['vlan'] and type(subnet['vlan']) == dict:
        copy['vlan'] = "%s:%s" % (subnet['vlan']['fabric'], subnet['vlan']['name'])
    return copy

# Deterine if two dictionaries are different
def different(have, want):
    have_keys = have.keys()
    for key in want.keys():
        if (key in have_keys and want[key] != have[key]) or key not in have_keys:
            debug.append({"have": have, "want": want, "key": key})
            return True
    return False

# Get a space object form MAAS based on its name
def get_space(maas, name):
    res = maas.get('/spaces/')
    if res.ok:
        for space in json.loads(res.text):
            if space['name'] == name:
                return space
    return None

# Get a fabric object from MAAS based on its name
def get_fabric(maas, name):
    res = maas.get('/fabrics/')
    if res.ok:
        for fabric in json.loads(res.text):
            if fabric['name'] == name:
                return fabric
    return None

# Get a VLAN object form MAAS based on its name
def get_vlan(maas, fabric, name ):
    res = maas.get('/fabrics/%d/vlans/' % fabric['id'])
    if res.ok:
        for vlan in json.loads(res.text):
            if vlan['name'] == name:
                return vlan
    return None

# Get an subnet from MAAS using its name, if not found return None
def get_subnet(maas, name):
    res = maas.get('/subnets/')
    if res.ok:
        for subnet in json.loads(res.text):
            if subnet['name'] == name:
                return simplify(subnet)
    return None

# Create an subnet based on the value given
def create_subnet(maas, subnet):
    merged = subnet.copy()
    # merged['op'] = 'new'
    res = maas.post('/subnets/', convert(maas, merged))
    if res.ok:
        return { 'error': False, 'status': get_subnet(maas, merged['name']) }
    return { 'error': True, 'status': string_or_object(res.text) }

# Delete an subnet based on the name
def delete_subnet(maas, name):
    res = maas.delete('/subnets/%s/' % name)
    if res.ok:
        return { 'error': False }
    return { 'error': True, 'status': string_or_object(res.text) }

def update_subnet(maas, have, want):
    merged = have.copy()
    merged.update(want)
    res = put(maas, '/subnets/%s/' % merged['id'], convert(maas, merged))
    if res.ok:
        return { 'error': False, 'status': get_subnet(maas, merged['name']) }
    return { 'error': True, 'status': string_or_object(res.text) }

def main():
    module = AnsibleModule(
        argument_spec = dict(
            maas=dict(default='http://localhost/MAAS/api/1.0/'),
            key=dict(required=True),
            name=dict(required=True),
            space=dict(required=False),
            dns_servers=dict(required=False),
            gateway_ip=dict(required=False),
            cidr=dict(required=False),
            state=dict(default='present', choices=['present', 'absent', 'query'])
        ),
        supports_check_mode = False
    )

    maas = module.params['maas']
    key = module.params['key']
    state = module.params['state']

    # Construct a sparsely populate desired state
    desired = remove_null({
        'name': module.params['name'],
        'space': module.params['space'],
        'dns_servers': module.params['dns_servers'],
        'gateway_ip': module.params['gateway_ip'],
        'cidr': module.params['cidr'],
    })

    # Authenticate into MAAS
    auth = MaasAuth(maas, key)
    maas = MaasClient(auth)

    # Attempt to get the subnet from MAAS
    subnet = get_subnet(maas, desired['name'])

    # Actions if the subnet does not currently exist
    if not subnet:
        if state == 'query':
            # If this is a query, returne it is not found
            module.exit_json(changed=False, found=False)
        elif state == 'present':
            # If this should be present, then attempt to create it
            res = create_subnet(maas, desired)
            if res['error']:
                module.fail_json(msg=res['status'])
            else:
                module.exit_json(changed=True, subnet=res['status'])
        else:
            # If this should be absent, then we are done and in the desired state
            module.exit_json(changed=False)

        # Done with subnets does not exists actions
        return

    # Actions if the subnet does exist
    if state == 'query':
        # If this is a query, return the subnet
        module.exit_json(changed=False, found=True, subnet=subnet)
    elif state == 'present':
        # If we want this to exists check to see if this is different and
        # needs updated
        if different(subnet, desired):
            res = update_subnet(maas, subnet, desired)
            if res['error']:
                module.fail_json(msg=res['status'])
            else:
                module.exit_json(changed=True, subnet=res['status'], debug=debug)
        else:
            # No differences, to nothing to change
            module.exit_json(changed=False, subnet=subnet)
    else:
        # If we don't want this subnet, then delete it
        res = delete_subnet(maas, subnet['name'])
        if res['error']:
            module.fail_json(msg=res['status'])
        else:
            module.exit_json(changed=True, subnet=subnet)

# this is magic, see lib/ansible/module_common.py
#<<INCLUDE_ANSIBLE_MODULE_COMMON>>
if __name__ == '__main__':
    main()
