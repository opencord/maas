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
module: maas_user
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
      - name of the user
    required: yes
  email:
    description:
      - email address of the user
    required: no
  password:
    description:
      - password for the user
    required: no
  is_superuser:
    description:
      - does the user have priviledges
    default: no
  state:
    description:
      - possible states for this user
    choices: ['present', 'absent', 'query']
    default: present

requirements: [ipaddress, requests_oauthlib, maasclient]
author: David Bainbridge
'''

EXAMPLES = '''
examples:
  maas_user:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MyUser
    email: user@company.com
    password: donttell
    is_superuser: no
    state: present

  maas_user:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MyDeadUser
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

# Get an user from MAAS using its name, if not found return None
def get_user(maas, name):
    res = maas.get('/users/%s/' % name)
    if res.ok:
        return json.loads(res.text)
    return None

# Create an user based on the value given
def create_user(maas, user):
    merged = user.copy()
    # merged['op'] = 'new'
    res = maas.post('/users/', merged)
    if res.ok:
        return { 'error': False, 'status': get_user(maas, merged['username']) }
    return { 'error': True, 'status': string_or_object(res.text) }

# Delete an user based on the name
def delete_user(maas, name):
    res = maas.delete('/users/%s/' % name)
    if res.ok:
        return { 'error': False }
    return { 'error': True, 'status': string_or_object(res.text) }

def main():
    module = AnsibleModule(
        argument_spec = dict(
            maas=dict(default='http://localhost/MAAS/api/1.0/'),
            key=dict(required=True),
            name=dict(required=True),
            email=dict(required=False),
            password=dict(required=False),
            is_superuser=dict(default=False, type='bool'),
            state=dict(default='present', choices=['present', 'absent', 'query'])
        ),
        supports_check_mode = False
    )

    maas = module.params['maas']
    key = module.params['key']
    state = module.params['state']

    # Construct a sparsely populate desired state
    desired = remove_null({
        'username': module.params['name'],
        'email': module.params['email'],
        'password': module.params['password'],
        'is_superuser': 0 if not module.params['is_superuser'] else 1
    })

    # Authenticate into MAAS
    auth = MaasAuth(maas, key)
    maas = MaasClient(auth)

    # Attempt to get the user from MAAS
    user = get_user(maas, desired['username'])

    # Actions if the user does not currently exist
    if not user:
        if state == 'query':
            # If this is a query, returne it is not found
            module.exit_json(changed=False, found=False)
        elif state == 'present':
            # If this should be present, then attempt to create it
            res = create_user(maas, desired)
            if res['error']:
                module.fail_json(msg=res['status'])
            else:
                module.exit_json(changed=True, user=res['status'])
        else:
            # If this should be absent, then we are done and in the desired state
            module.exit_json(changed=False)

        # Done with users does not exists actions
        return

    # Actions if the user does exist
    if state == 'query':
        # If this is a query, return the user
        module.exit_json(changed=False, found=True, user=user)
    elif state == 'present':
        # If we want this to exists check to see if this is different and
        # needs updated
        if different(user, desired):
            module.fail_json(msg='Specified user, "%s", exists and MAAS does not allow the user to be modified programatically'
                    % user['username'])
        else:
            # No differences, to nothing to change
            module.exit_json(changed=False, user=user)
    else:
        # If we don't want this user, then delete it
        res = delete_user(maas, user['username'])
        if res['error']:
            module.fail_json(msg=res['status'])
        else:
            module.exit_json(changed=True, user=user)

# this is magic, see lib/ansible/module_common.py
#<<INCLUDE_ANSIBLE_MODULE_COMMON>>
if __name__ == '__main__':
    main()
