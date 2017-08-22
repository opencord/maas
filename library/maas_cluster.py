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
module: maas_cluster
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
      - name of the cluster
    required: yes
  status:
    description:
      - indicates the enabled state of the cluster
    choices: ['enabled', 'disabled']
    default: enabled
  domain:
    description:
      - DNS zone name
    required: no
  state:
    description:
      - possible states for this cluster
    choices: ['present', 'query']
    default: present

requirements: [ipaddress, requests_oauthlib, maasclient]
author: David Bainbridge
'''

EXAMPLES = '''
examples:
  maas_cluster:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MyCluster
    status: enabled
    domain: company.com
    state: present

  maas_cluster:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    name: MyDeadCluster
    state: query
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

# Get an cluster from MAAS using its name, if not found return None
def get_cluster(maas, name):
    res = maas.get('/nodegroups/', dict(op='list'))
    if res.ok:
        for ng in json.loads(res.text):
            if ng['cluster_name'] == name:
                return ng
    return None

def update_cluster(maas, have, want):
    merged = have.copy()
    merged.update(want)
    res = put(maas, '/nodegroups/%s/' % merged['uuid'], merged)
    if res.ok:
        return { 'error': False, 'status': get_cluster(maas, merged['cluster_name']) }
    return { 'error': True, 'status': string_or_object(res.text) }

def main():
    module = AnsibleModule(
        argument_spec = dict(
            maas=dict(default='http://localhost/MAAS/api/1.0/'),
            key=dict(required=True),
            name=dict(required=True),
            status=dict(default='enabled', choices=['enabled', 'disabled']),
            domain=dict(required=False),
            state=dict(default='present', choices=['present', 'query'])
        ),
        supports_check_mode = False
    )

    maas = module.params['maas']
    key = module.params['key']
    state = module.params['state']

    status_map = {
        'enabled': 1,
        'disabled': 2
    }

    # Construct a sparsely populate desired state
    desired = remove_null({
        'cluster_name': module.params['name'],
        'status': status_map[module.params['status']],
        'name' : module.params['domain'],
    })

    # Authenticate into MAAS
    auth = MaasAuth(maas, key)
    maas = MaasClient(auth)

    # Attempt to get the cluster from MAAS
    cluster = get_cluster(maas, desired['cluster_name'])

    # Actions if the cluster does not currently exist
    if not cluster:
        if state == 'query':
            # If this is a query, returne it is not found
            module.exit_json(changed=False, found=False)
        elif state == 'present':
            # Not able to create clusters via the API
            module.fail_json(msg='Named cluster does not exist and clusters cannot be programatically created')
        else:
            # If this should be absent, then we are done and in the desired state
            module.exit_json(changed=False)

        # Done with clusters does not exists actions
        return

    # Actions if the cluster does exist
    if state == 'query':
        # If this is a query, return the cluster
        module.exit_json(changed=False, found=True, cluster=cluster)
    elif state == 'present':
        # If we want this to exists check to see if this is different and
        # needs updated
        if different(cluster, desired):
            res = update_cluster(maas, cluster, desired)
            if res['error']:
                module.fail_json(msg=res['status'])
            else:
                module.exit_json(changed=True, cluster=res['status'])
        else:
            # No differences, to nothing to change
            module.exit_json(changed=False, cluster=cluster)
    else:
        # Not able to delete clusters via the API
        module.fail_json(msg='Named cluster exists and clusters cannot be programmatically deleted')

# this is magic, see lib/ansible/module_common.py
#<<INCLUDE_ANSIBLE_MODULE_COMMON>>
if __name__ == '__main__':
    main()
