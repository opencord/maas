#!/bin/bash

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

function verify {
    local L=$1
    for i in $L; do
        grep $i /etc/bind/maas/dhcp_harvest.inc > /dev/null 2>&1
        if [ $? -ne 0 ]; then
            echo "0"
            return
        fi
    done
    echo "1"
}

for i in $(uvt-kvm list); do
    virsh start $i
done

LIST=$(uvt-kvm list)
CNT=$(uvt-kvm list | wc -l)
# plus 4 for the switches

RETRY=5
VERIFIED=0
while [ $VERIFIED -ne 1 -a $RETRY -gt 0 ]; do
    echo "INFO: Waiting for VMs to start"
    sleep 5
    curl -slL -XPOST http://127.0.0.1:8954/harvest >> /dev/null
    VERIFIED=$(verify $LIST)
    RETRY=$(expr $RETRY - 1)
    echo "INFO: Verifing all VMs started"
done

if [ $VERIFIED -ne 1 ]; then
    echo "ERROR: Likely VMs did not all boot correctly"
    exit 1
else
    echo "INFO: Looks like all VM started correctly"
fi
