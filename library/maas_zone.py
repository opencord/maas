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
module: maas_zone
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
      - name of the zone
    required: yes
  description:
    description:
      - description text of zone
    required: no
  state:
    description:
      - possible states for this zone
    choices: ['present', 'absent', 'query']
    default: present

requirements: [ipaddress, requests_oauthlib, maasclient]
author: David Bainbridge
'''

EXAMPLES = '''
examples:
  maas_zone:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MyZone
    description: This is my zone
    state: present

  maas_zone:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MyDeadZone
    description: This was my zone
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
def different(have, want):
    have_keys = have.keys()
    for key in want.keys():
        if (key in have_keys and want[key] != have[key]) or key not in have_keys:
            return True
    return False

# Get an zone from MAAS using its name, if not found return None
def get_zone(maas, name):
    res = maas.get('/zones/%s/' % name)
    if res.ok:
        return json.loads(res.text)
    return None

# Create an zone based on the value given
def create_zone(maas, zone):
    merged = zone.copy()
    # merged['op'] = 'new'
    res = maas.post('/zones/', merged)
    if res.ok:
        return { 'error': False, 'status': get_zone(maas, merged['name']) }
    return { 'error': True, 'status': string_or_object(res.text) }

# Delete an zone based on the name
def delete_zone(maas, name):
    res = maas.delete('/zones/%s/' % name)
    if res.ok:
        return { 'error': False }
    return { 'error': True, 'status': string_or_object(res.text) }

def update_zone(maas, have, want):
    merged = have.copy()
    merged.update(want)
    res = put(maas, '/zones/%s/' % merged['name'], merged)
    if res.ok:
        return { 'error': False, 'status': get_zone(maas, merged['name']) }
    return { 'error': True, 'status': string_or_object(res.text) }

def main():
    module = AnsibleModule(
        argument_spec = dict(
            maas=dict(default='http://localhost/MAAS/api/1.0/'),
            key=dict(required=True),
            name=dict(required=True),
            description=dict(required=False),
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
        'description': module.params['description'],
    })

    # Authenticate into MAAS
    auth = MaasAuth(maas, key)
    maas = MaasClient(auth)

    # Attempt to get the zone from MAAS
    zone = get_zone(maas, desired['name'])

    # Actions if the zone does not currently exist
    if not zone:
        if state == 'query':
            # If this is a query, returne it is not found
            module.exit_json(changed=False, found=False)
        elif state == 'present':
            # If this should be present, then attempt to create it
            res = create_zone(maas, desired)
            if res['error']:
                module.fail_json(msg=res['status'])
            else:
                module.exit_json(changed=True, zone=res['status'])
        else:
            # If this should be absent, then we are done and in the desired state
            module.exit_json(changed=False)

        # Done with zones does not exists actions
        return

    # Actions if the zone does exist
    if state == 'query':
        # If this is a query, return the zone
        module.exit_json(changed=False, found=True, zone=zone)
        return
    elif state == 'present':
        # If we want this to exists check to see if this is different and
        # needs updated
        if different(zone, desired):
            res = update_zone(maas, zone, desired)
            if res['error']:
                module.fail_json(msg=res['status'])
            else:
                module.exit_json(changed=True, zone=res['status'])
        else:
            # No differences, to nothing to change
            module.exit_json(changed=False, zone=zone)
    else:
        # If we don't want this zone, then delete it
        res = delete_zone(maas, zone['name'])
        if res['error']:
            module.fail_json(msg=res['status'])
        else:
            module.exit_json(changed=True, zone=zone)

# this is magic, see lib/ansible/module_common.py
#<<INCLUDE_ANSIBLE_MODULE_COMMON>>
if __name__ == '__main__':
    main()
