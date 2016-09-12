#!/usr/bin/env python

import sys
import json
import ethtool
import shlex

# read the argument string from the arguments file
args_file = sys.argv[1]
args_data = file(args_file).read()

ignore=["tun", "bridge", "bonding", "veth"]
bus_ignore=["", "N/A", "tap"]

# parse the task options
arguments = shlex.split(args_data)
for arg in arguments:
    # ignore any arguments without an equals in it
    if "=" in arg:
        (key, value) = arg.split("=")
    # if setting the time, the key 'time'
    # will contain the value we want to set the time to

all = {}
for i in ethtool.get_devices():
    o = { "name": i }
    try:
        module = ethtool.get_module(i)
        businfo = ethtool.get_businfo(i)
        if module in ignore or businfo in bus_ignore:
            continue
        all[i] = {
            "name": i,
            "module" : module,
        }
    except:
        pass

print json.dumps({
    "changed" : False,
    "ansible_facts" : {
        "netinfo" : all,
    },
})
