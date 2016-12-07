#!/bin/bash

HOSTS="10.3.1.1 10.3.1.2 10.3.2.1 10.3.2.2 10.3.1.254 10.3.2.254 192.168.10.1 8.8.8.8"

ME=$(ifconfig | grep "10\.3\.[0-9]\.[0-9]" | sed -e 's/.*addr:\(10\.3\.[0-9]\.[0-9]\).*/\1/g' 2> /dev/null)
echo "FROM: $ME"
for TO in $HOSTS; do
    T=$(ping -q -c 1 -W 1 -I eth0 $TO | grep rtt | awk '{print $4}' | sed -e 's|/| |g') #sed -e 's|r| |')
    echo  "$TO: $T" | awk '{printf("    %-15s %-7s\n", $1, $2, $3, $4)}'
done
