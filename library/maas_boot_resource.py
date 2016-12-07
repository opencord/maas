#!/usr/bin/python

DOCUMENTATION = '''
---
module: maas_boot_resources
short_description: Manage MAAS boot resources
options:
  maas:
    description:
      - URL of MAAS server
    default: http://localhost/MAAS/api/1.0/
  key:
    description:
      - MAAS API key
    required: yes
  state:
    description:
      - possible states for this sshkey
    choices: ['query', 'import']
    default: query

requirements: [ipaddress, requests_oauthlib, maasclient]
author: David Bainbridge
'''

EXAMPLES = '''
examples:
  maas_boot_resource:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
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

def filter(filter_type, d, keys):
    if filter_type == 'include':
        for k in d.keys():
            if k not in keys:
                d.pop(k, None)
    else:
        for k in d.keys():
            if k in keys:
                d.pop(k, None)

def main():
    module = AnsibleModule(
        argument_spec = dict(
            maas=dict(default='http://localhost/MAAS/api/1.0/'),
            key=dict(required=True),
            state=dict(default='query', choices=['query', 'import'])
        ),
        supports_check_mode = False
    )

    maas = module.params['maas']
    key = module.params['key']
    state = module.params['state']

    # Authenticate into MAAS
    auth = MaasAuth(maas, key)
    maas = MaasClient(auth)

    if state == 'query':
        res = maas.get('/boot-resources/')
        if res.ok:
            module.exit_json(changed=False, resources=json.loads(res.text))
        else:
            module.fail_json(msg=string_or_object(res.text))
    elif state == 'import':
        res = maas.post('/boot-resources/', dict(op='import'))
        if res.ok:
            module.exit_json(changed=True)
        else:
            module.fail_json(msg=string_or_object(res.text))
    else:
        module.fail_json(msg='unknown state')

# this is magic, see lib/ansible/module_common.py
#<<INCLUDE_ANSIBLE_MODULE_COMMON>>
if __name__ == '__main__':
    main()
