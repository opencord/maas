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

import json
import os
import re
import sys
import shlex
import string
import ipaddress
# WANT_JSON

# Regular expressions to identify comments and blank lines
comment = re.compile("^\s*#")
blank = re.compile("^\s*$")

#####
# Parsers
#
# Parses are methods that take the form 'parse_<keyword>', where the keyword
# is the first word on a line in file. The purpose of the parser is to
# evaluate the line tna update the interface model accordingly.
####

# Compares the current and desired network configuration to see if there
# is a change and returns:
# 0 if no change
# -1 if the change has no semantic value (i.e. comments differ)
# 1 if there is a semantic change (i.e. its meaningful)
# the highest priority of change is returned, i.e. if there is both
# a semantic and non-semantic change a 1 is returned indicating a
# semantic change.
def value_equal(left, right):
    if type(left) == type(right):
        return left == right
    return str(left) == str(right)

def compare(have, want):
    result = 0
    for key in list(set().union(have.keys(), want.keys())):
        if key in have.keys() and key in want.keys():
            if not value_equal(have[key], want[key]):
                if key in ["description"]:
                    result = -1
                else:
                    return 1
        else:
            if key in ["description"]:
                result = -1
            else:
                return 1
    return result

# Creates an interface definition in the model and sets the auto
# configuration to true
def parse_auto(data, current, words, description):
    if words[1] in data.keys():
        iface = data[words[1]]
    else:
        iface = {}

    if len(description) > 0:
        iface["description"] = description

    iface["auto"] = True
    data[words[1]] = iface
    return words[1]

# Creates an interface definition in the model if one does not exist and
# sets the type and configuation method
def parse_iface(data, current, words, description):
    if words[1] in data.keys():
        iface = data[words[1]]
    else:
        iface = {}

    if len(description) > 0:
        iface["description"] = description

    iface["type"] = words[2]
    iface["config"] = words[3]
    data[words[1]] = iface
    return words[1]

allow_lists = ["pre-up", "post-up", "pre-down", "post-down"]

# Used to evaluate attributes and add a generic name / value pair to the interface
# model
def parse_add_attr(data, current, words, description):
    global allow_lists
    if current == "":
        raise SyntaxError("Attempt to add attribute '%s' without an interface" % words[0])

    if current in data.keys():
        iface = data[current]
    else:
        iface = {}

    if len(description) > 0:
        iface["description"] = description

    if words[0] in iface and words[0] in allow_lists:
        have = iface[words[0]]
        if type(have) is list:
            iface[words[0]].append(" ".join(words[1:]))
        else:
            iface[words[0]] = [have, " ".join(words[1:])]
    else:
        iface[words[0]] = " ".join(words[1:])

    data[current] = iface
    return current

#####
# Writers
#
# Writers take the form of 'write_<keyword>` where keyword is an interface
# attribute. The role of the writer is to output the attribute to the
# output stream, i.e. the new interface file.
#####

# Writes a generic name / value pair indented
def write_attr(out, name, value):
    if isinstance(value, list):
        for line in value:
            out.write("  %s %s\n" % (name, line))
    else:
        out.write("  %s %s\n" % (name, value))

# Writes an interface definition to the output stream
def write_iface(out, name, iface):
    if "description" in iface.keys():
        val = iface["description"]
        if len(val) > 0 and val[0] != "#":
            val = "# " + val
        out.write("%s\n" % (val))
    if "auto" in iface.keys() and iface["auto"]:
        out.write("auto %s\n" % (name))
    out.write("iface %s %s %s\n" % (name, iface["type"], iface["config"]))
    for attr in sorted(iface.keys(), key=lambda x:x in write_sort_order.keys() and write_sort_order[x] or 100):
        if attr in write_ignore:
            continue
        writer = "write_%s" % (attr)
        if writer in all_methods:
            globals()[writer](out, attr, iface[attr])
        else:
            write_attr(out, attr, iface[attr])
    out.write("\n")

# Writes the new interface file
def write(out, data):
#    out.write("# This file describes the network interfaces available on your system\n")
#    out.write("# and how to activate them. For more information, see interfaces(5).\n\n")
    # First to loopback
    for name, iface in data.items():
        if iface["config"] != "loopback":
            continue
        write_iface(out, name, iface)

    for iface in sorted(data.keys(), key=lambda x:x in write_iface_sort_order.keys() and write_iface_sort_order[x] or x):
        if data[iface]["config"] == "loopback":
            continue
        write_iface(out, iface, data[iface])

# The defaults for the netfile task
src_file = "/etc/network/interfaces"
dest_file = None
merge_comments = False
state = "present"
name = ""
force = False
values = {
    "config": "manual",
    "type": "inet"
}

# read the argument string from the arguments file
args_file = sys.argv[1]
args_data = file(args_file).read()

# parse the task options
arguments = json.loads(args_data)
for key, value in arguments.iteritems():
    if key == "src":
        src_file = value
    elif key == "dest":
        dest_file = value
    elif key == "name":
        name = value
    elif key == "state":
        state = value
    elif key == "force":
        force = value.lower() in ['true', 't', 'yes', 'y']
    elif key == "description":
        values["description"] = value
    elif key == "merge-comments":
        merge_comments = value.lower() in ['true', 't', 'yes', 'y']
    elif key == "address":
        if string.find(value, "/") != -1:
            parts = value.split('/')
            addr = ipaddress.ip_network(value, strict=False)
            values["address"] = parts[0]
            values["network"] = addr.network_address.exploded.encode('ascii','ignore')
            values["netmask"] = addr.netmask.exploded.encode('ascii','ignore')
            values["broadcast"] = addr.broadcast_address.exploded.encode('ascii','ignore')
        else:
            values["address"] = value
    elif key[0] != '_':
        values[key] = value

# If name is not set we need to error out
if name == "":
    result = {
        "changed": False,
        "failed": True,
        "msg": "Name is a mansitory parameter",
    }
    print json.dumps(result)
    sys.stdout.flush()
    exit(1)

# If no destination file was specified, write it back to the same file
if not dest_file:
    dest_file = src_file

# all methods is used to check if parser or writer methods exist
all_methods = dir()

# which attributes should be ignored and not be written as single
# attributes values against and interface
write_ignore = ["auto", "type", "config", "description", "source"]

# specifies the order in which attributes are written against an
# interface. Any attribute note in this list is sorted by default
# order after the attributes specified.
write_sort_order = {
    "address"   :  1,
    "network"   :  2,
    "netmask"   :  3,
    "broadcast" :  4,
    "gateway"   :  5,
    "pre-up"    : 10,
    "post-up"   : 11,
    "pre-down"  : 12,
    "post-down" : 13
}

write_iface_sort_order = {
    "fabric" : "y",
    "mgmtbr" : "z"
}

# Read and parse the specified interface file
file = open(src_file, "r")
ifaces = {}
current = "" # The current interface being parsed
description = ""
for line in file.readlines():
    line = line.rstrip('\n')

    if comment.match(line):
        if len(description) > 0:
            description = description + '\n' + line
        else:
            description = line

    if len(description) > 0 and blank.match(line):
        description = description + '\n'

    # Drop any comment of blank line
    if comment.match(line) or blank.match(line):
        continue

    # Parse the line
    words = line.split()
    parser = "parse_" + words[0].replace("-", "_")
    if parser in all_methods:
        current = globals()[parser](ifaces, current, words, description)
    else:
        current = parse_add_attr(ifaces, current, words, description)

    description = ""

file.close()

# Assume no change unless we discover otherwise
result = {
    "changed" : False
}
change_type = 0

# if the interface specified and state is present then either add
# it to the model or replace it if it already exists.
if state == "query":
    if name in ifaces.keys():
        result["interface"] = ifaces[name]
        result["found"] = True
    else:
        result["found"] = False
elif state == "present":
    if name in ifaces.keys():
        have = ifaces[name]
        change_type = compare(have, values)
        result["change_type"] = change_type
        if change_type != 0:
            ifaces[name] = values
            if merge_comments and "description" in have.keys() and len(have["description"]) > 0:
                result["merge_comments"] = True
                if "description" in values.keys() and len(values["description"]) > 0:
                    ifaces[name]["description"] = values["description"] + "\n" + have["description"]
                else:
                    ifaces[name]["description"] = have["description"]
            result["changed"] = (change_type == 1)
    else:
        ifaces[name] = values
        result["changed"] = True


# if state is absent then remove it from the model
elif state == "absent" and name in ifaces.keys():
    del ifaces[name]
    result["changed"] = True

# Only write the output file if something has changed or if the
# task requests a forced write.
if force or result["changed"] or change_type != 0:
    file = open(dest_file, "w+")
    write(file, ifaces)
    file.close()

# Output the task result
print json.dumps(result)
