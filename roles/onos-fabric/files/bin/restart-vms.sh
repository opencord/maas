#!/bin/bash

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
