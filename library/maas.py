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
module: maas
short_description: Manage MAAS server configuration
options:
  maas:
    description:
      - URL of MAAS server
    default: http://localhost/MAAS/api/1.0/
  key:
    description:
      - MAAS API key
    required: yes
  options:
    description:
      - list of config options to query, this is only used for query
  enable_http_proxy: :
    description:
      - Enable the use of an APT and HTTP/HTTPS proxy.
  upstream_dns: :
    description:
      - Upstream DNS used to resolve domains not managed by this MAAS (space-separated IP addresses).
  default_storage_layout: :
    description:
      - Default storage layout.
    choices:
      - ['lvm', 'flat', 'bcache']
  default_osystem: :
    description:
      - Default operating system used for deployment.
  ports_archive: :
    description:
      - Ports archive.
  http_proxy: :
    description:
      - Proxy for APT and HTTP/HTTPS.
  boot_images_auto_import: :
    description:
      - Automatically import/refresh the boot images every 60 minutes.
  enable_third_party_drivers: :
    description:
      - Enable the installation of proprietary drivers (i.e. HPVSA).
  kernel_opts: :
    description:
      - Boot parameters to pass to the kernel by default.
  main_archive: :
    description:
      - Main archive
  maas_name: :
    description:
      - MAAS name.
  curtin_verbose: :
    description:
      - Run the fast-path installer with higher verbosity. This provides more detail in the installation logs..
  dnssec_validation: :
    description:
      - Enable DNSSEC validation of upstream zones.
  commissioning_distro_series: :
    description:
      - Default Ubuntu release used for commissioning.
  windows_kms_host: :
    description:
      - Windows KMS activation host.
  enable_disk_erasing_on_release: :
    description:
      - Erase nodes' disks prior to releasing..
  default_distro_series: :
    description:
      - Default OS release used for deployment.
  ntp_server: :
    description:
      - Address of NTP server for nodes.
  default_min_hwe_kernel: :
    description:
      - Default Minimum Kernel Version.
  state:
    description:
      - possible states for the module
    choices: ['present', 'query']
    default: present

requirements: [ipaddress, requests_oauthlib, maasclient]
author: David Bainbridge
'''

EXAMPLES = '''
examples:
  maas:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    options:
      - upstream_dns
      - ntp_servers
    state: query

  maas:
    maas: http://my.maas.server.com/MAAS/api/1.0/
    key: 'xBvr9dx5k7S52myufC:fqBXV7hJgXegNZDw9c:K8hsmL47XjAppfQy2pDVW7G49p6PELgp'
    upstream_dns: 8.8.8.8 8.8.8.4
    state: present
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

# Get configuration options from MAAS
def get_config(maas, desired):
    config = {}
    for o in desired.keys():
        res = maas.get('/maas/', dict(name=o, op='get_config'))
        if res.ok:
            val = json.loads(res.text)
            config[o] = val if val else ""
        else:
            config[o] = {'error': string_or_object(res.text)}
    return config

# Walk the list of options in the desired state setting those on MAAS
def update_config(maas, have, want):
    have_error = False
    status = {}
    for o in want.keys():
        if want[o] != have[o]:
            res = maas.post('/maas/', {'name': o, 'value': want[o], 'op': 'set_config'})
            if res.ok:
                status[o] = { 'error': False, 'status': want[o] }
            else:
                have_error = True
                status[o] = { 'error': True, 'status': string_or_object(res.text) }
    return {'error': have_error, 'status': status}

def main():
    module = AnsibleModule(
        argument_spec = dict(
            maas=dict(default='http://localhost/MAAS/api/1.0/'),
            key=dict(required=True),
            options=dict(required=False, type='list'),
            enable_http_proxy=dict(required=False),
            upstream_dns=dict(required=False),
            default_storage_layout=dict(required=False),
            default_osystem=dict(required=False),
            ports_archive=dict(required=False),
            http_proxy=dict(required=False),
            boot_images_auto_import=dict(required=False),
            enable_third_party_drivers=dict(required=False),
            kernel_opts=dict(required=False),
            main_archive=dict(required=False),
            maas_name=dict(required=False),
            curtin_verbose=dict(required=False),
            dnssec_validation=dict(required=False),
            commissioning_distro_series=dict(required=False),
            windows_kms_host=dict(required=False),
            enable_disk_erasing_on_release=dict(required=False),
            default_distro_series=dict(required=False),
            ntp_server=dict(required=False),
            default_min_hwe_kernel=dict(required=False),
            state=dict(default='present', choices=['present', 'query'])
        ),
        supports_check_mode = False
    )

    maas = module.params['maas']
    key = module.params['key']
    options = module.params['options']
    state = module.params['state']

    if state == 'query':
        desired = {x:None for x in options}
    else:
        # Construct a sparsely populate desired state
        desired = remove_null({
            'enable_http_proxy': module.params['enable_http_proxy'],
            'upstream_dns': module.params['upstream_dns'],
            'default_storage_layout': module.params['default_storage_layout'],
            'default_osystem': module.params['default_osystem'],
            'ports_archive': module.params['ports_archive'],
            'http_proxy': module.params['http_proxy'],
            'boot_images_auto_import': module.params['boot_images_auto_import'],
            'enable_third_party_drivers': module.params['enable_third_party_drivers'],
            'kernel_opts': module.params['kernel_opts'],
            'main_archive': module.params['main_archive'],
            'maas_name': module.params['maas_name'],
            'curtin_verbose': module.params['curtin_verbose'],
            'dnssec_validation': module.params['dnssec_validation'],
            'commissioning_distro_series': module.params['commissioning_distro_series'],
            'windows_kms_host': module.params['windows_kms_host'],
            'enable_disk_erasing_on_release': module.params['enable_disk_erasing_on_release'],
            'default_distro_series': module.params['default_distro_series'],
            'ntp_server': module.params['ntp_server'],
            'default_min_hwe_kernel': module.params['default_min_hwe_kernel'],
        })

    # Authenticate into MAAS
    auth = MaasAuth(maas, key)
    maas = MaasClient(auth)

    # Attempt to get the configuration from MAAS
    config = get_config(maas, desired)

    if state == 'query':
        # If this is a query, return the options
        module.exit_json(changed=False, found=True, maas=config)
    elif state == 'present':
        # If we want this to exists check to see if this is different and
        # needs updated
        if different(config, desired):
            res = update_config(maas, config, desired)
            if res['error']:
                module.fail_json(msg=res['status'])
            else:
                module.exit_json(changed=True, maas=res['status'])
        else:
            # No differences, to nothing to change
            module.exit_json(changed=False, maas=config)

# this is magic, see lib/ansible/module_common.py
#<<INCLUDE_ANSIBLE_MODULE_COMMON>>
if __name__ == '__main__':
    main()
