#!/usr/bin/env python

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

import sys
import json
import ethtool
import shlex
import re

def match_list(value, list):
    if len(list) == 0:
        return False

    for exp in list:
        if re.match(exp, value):
            return True

    return False

def has_match(name, module_type, bus_type, name_list, module_type_list, bus_type_list):
    return match_list(name, name_list) or \
                    match_list(module_type, module_type_list) or match_list(bus_type, bus_type_list)

# read the argument string from the arguments file
args_file = sys.argv[1]
args_data = file(args_file).read()

exclude_names=[]
exclude_module_types=[]
exclude_bus_types=[]
include_names=[]
include_module_types=[]
include_bus_types=[]
ignore_names=[]
ignore_module_types=["tun", "bridge", "bonding", "veth"]
ignore_bus_types=["^\W*$", "N/A", "tap"]
debug_out = False

# parse the task options
arguments = shlex.split(args_data)
for arg in arguments:
    # exclude any arguments without an equals in it
    if "=" in arg:
        (key, value) = arg.split("=")

    if key == "exclude-names":
        exclude_names = re.split("\W*,\W*", value)
    elif key == "exclude-module-types":
        exclude_module_types = re.split("\W*,\W*", value)
    elif key == "exclude-bus-types":
        exclude_bus_types = re.split("\W*,\W*", value)
    elif key == "include-names":
        include_names = re.split("\W*,\W*", value)
    elif key == "include-module-types":
        include_module_types = re.split("\W*,\W*", value)
    elif key == "include-bus-types":
        include_bus_types = re.split("\W*,\W*", value)
    elif key == "ignore-names":
        ignore_names = re.split("\W*,\W*", value)
    elif key == "ignore-module-types":
        ignore_module_types = re.split("\W*,\W*", value)
    elif key == "ignore-bus-types":
        ignore_bus_types = re.split("\W*,\W*", value)
    elif key == "debug":
        debug_out = value.lower() in ["true", "yes", "on", "t", "y", "1"]
    elif key[0] != '_':
        raise ValueError('Unknown option to task "%s"' % key)

included = {}
ignored = {}
excluded = {}
debug = []

if debug_out:
    debug.append("EXCLUDES: '%s', '%s', '%s'" % (exclude_names, exclude_module_types, exclude_bus_types))
    debug.append("INCLUDE: '%s', '%s', '%s'" % (include_names, include_module_types, include_bus_types))
    debug.append("IGNORE: '%s', '%s', '%s'" % (ignore_names, ignore_module_types, ignore_bus_types))

for i in ethtool.get_devices():
    o = { "name": i }
    try:
        module = ethtool.get_module(i)
        businfo = ethtool.get_businfo(i)

        # If it matches an ignore pattern then just ignore it.
        if has_match(i, module, businfo, ignore_names, ignore_module_types, ignore_bus_types):
            if debug_out: debug.append("IGNORE '%s' on ignore match" % i)
            ignored[i] = {
                "name": i,
                "module": module,
            }
            continue

        # If no include specifications have been set and the interface is not ignored
        # it needs to be considered for inclusion
        if len(include_names) + len(include_module_types) + len(include_bus_types) == 0:
            # If it matches exclude list then exclude it, else include it
            if has_match(i, module, businfo, exclude_names, exclude_module_types, exclude_bus_types):
                if debug_out: debug.append("EXCLUDE '%s' with no include specifiers, but with exclude match" %i)
                excluded[i] = {
                    "name": i,
                    "module": module,
                }
                continue
            if debug_out: debug.append("INCLUDE '%s' with no include specifiers, but with no exclude match" % i)
            included[i] = {
                "name": i,
                "module": module,
            }
            continue

        # If any of the include specifications are set then the interface must match at least one
        # to be added to the mached list.
        if has_match(i, module, businfo, include_names, include_module_types, include_bus_types):
            if debug_out: debug.append("MATCH '%s' has include match" % i)
            # If it matches exclude list then exclude it, else include it
            if has_match(i, module, businfo, exclude_names, exclude_module_types, exclude_bus_types):
                if debug_out: debug.append("EXCLUDE '%s' with include match, but also with exclude match" % i)
                excluded[i] = {
                    "name": i,
                    "module": module,
                }
                continue
            if debug_out: debug.append("INCLUDE '%s' with include match and with no exclude match" % i)
            included[i] = {
                "name": i,
                "module" : module,
            }
            continue

        # Implicitly ignore
        if debug_out: debug.append("IGNORE: '%s' implicitly" %i)
        ignored[i] = {
            "name": i,
            "module": module,
        }

    except:
        pass

result = {
    "changed" : False,
    "ansible_facts" : {
        "netinfo" : {
            "included" : included,
            "excluded" : excluded,
            "ignored"  : ignored,
        },
    },
}

if debug_out: result["ansible_facts"]["netinfo"]["debug"] = debug

print json.dumps(result)
