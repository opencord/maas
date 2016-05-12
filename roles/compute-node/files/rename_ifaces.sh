#!/bin/bash

function ip2int {
    local a b c d
    { IFS=. read a b c d; } <<< $1
    echo $(((((((a << 8) | b) << 8) | c) << 8) | d))
}

function int2ip {
    local ui32=$1; shift
    local ip n
    for n in 1 2 3 4; do
        ip=$((ui32 & 0xff))${ip:+.}$ip
        ui32=$((ui32 >> 8))
    done
    echo $ip
}

function netmask {
    local mask=$((0xffffffff << (32 - $1))); shift
    int2ip $mask
}

function broadcast {
    local addr=$(ip2int $1); shift
    local mask=$((0xffffffff << (32 -$1))); shift
    int2ip $((addr | ~mask))
}

function network {
    local addr=$(ip2int $1); shift
    local mask=$((0xffffffff << (32 -$1))); shift
    int2ip $((addr & mask))
}

function first {
    local addr=$(ip2int $1)
    addr=`expr $addr + 1`
    int2ip $addr
}

function guess_type {
    local CNT=$(echo "$1" | sed -e 's/[:.]/ /g' | wc -w)
    if [ $CNT -ne 1 ]; then
        # drop all sub and vlan interfaces
        echo "DNC"
        return
    fi
    local DRIVER=$(ethtool -i $1 2>/dev/null | grep driver | awk '{print $2}')
    local RESULT="DNC"
    case $DRIVER in
        i40e)
            RESULT="I40G"
            ;;
        igb|e1000)
            RESULT="ETH"
            ;;
        *) ;;
    esac
    echo $RESULT
}

function get_mac {
  echo $(ifconfig $1 | grep HWaddr | awk '{print $5}')
}

function generate_persistent_names {
    local OUT=$NAMES_FILE
#"70-persistent-net.rules"
    rm -rf $OUT

    IDX=0
    for i in $(cat $1 | sort); do
        echo "SUBSYSTEM==\"net\", ACTION==\"add\", DRIVERS==\"?*\", ATTR{address}==\"$i\", ATTR{dev_id}==\"0x0\", ATTR{type}==\"1\", KERNEL==\"eth*\", NAME=\"eth$IDX\"" >> $OUT
        IDX=$(expr $IDX + 1)
    done

    for i in $(cat $2 | sort); do
        echo "SUBSYSTEM==\"net\", ACTION==\"add\", DRIVERS==\"?*\", ATTR{address}==\"$i\", NAME=\"eth$IDX\"" >> $OUT
        IDX=$(expr $IDX + 1)
    done
}

function generate_interfaces {
    OUT=$IFACES_FILE
    rm -rf $OUT
    echo "# This file describes the network interfaces available on your system" >> $OUT
    echo "# and how to activate them. For more information, see interfaces(5)." >> $OUT
    echo "" >> $OUT
    echo "# The loopback network interface" >> $OUT
    echo "auto lo" >> $OUT
    echo "iface lo inet loopback" >> $OUT
    echo "" >> $OUT

    IDX=0
    FIRST=1
    for i in $(cat $1); do
        if [ $FIRST -eq 1 ]; then
            echo "auto eth$IDX" >> $OUT
            echo "iface eth$IDX inet static" >> $OUT
            echo "    address $IP" >> $OUT
            echo "    network $NETWORK" >> $OUT
            echo "    netmask $NETMASK" >> $OUT
            FIRST=0
        else
            echo "iface eth$IDX inet manual" >> $OUT
        fi
        echo "" >> $OUT
        IDX=$(expr $IDX + 1)
    done

    FIRST=1
    for i in $(cat $2); do
        if [ $FIRST -eq 1 ]; then
            echo "auto eth$IDX" >> $OUT
            echo "iface eth$IDX inet dhcp" >> $OUT
            FIRST=0
        else
            echo "iface eth$IDX inet manual" >> $OUT
        fi
        echo "" >> $OUT
        IDX=$(expr $IDX + 1)
    done
}

ADDR=$1
IP=$(echo $ADDR | cut -d/ -f1)
MASKBITS=$(echo $ADDR | cut -d/ -f2)
NETWORK=$(network $IP $MASKBITS)
NETMASK=$(netmask $MASKBITS)

LIST_ETH=$(mktemp -u)
LIST_40G=$(mktemp -u)
IFACES_FILE=$(mktemp -u)
NAMES_FILE=$(mktemp -u)

IFACES=$(ifconfig -a | grep "^[a-z]" | awk '{print $1}')

for i in $IFACES; do
    TYPE=$(guess_type $i)
    case $TYPE in
        ETH)
            echo "$(get_mac $i)" >> $LIST_ETH
            ;;
        I40G)
            echo "$(get_mac $i)" >> $LIST_40G
            ;;
        *) ;;
    esac
done

RESULT="false"

generate_interfaces $LIST_40G $LIST_ETH
diff /etc/network/interfaces $IFACES_FILE 2>&1 > /dev/null
if [ $? -ne 0 ]; then
  RESULT="true"
  cp /etc/network/interfaces /etc/network/interfaces.1
  cp $IFACES_FILE /etc/network/interfaces
fi

generate_persistent_names $LIST_40G $LIST_ETH
if [ -r /etc/udev/rules.d/70-persistent-net.rules ]; then
  diff /etc/udev/rules.d/70-persistent-net.rules $NAMES_FILE 2>&1 > /dev/null
  if [ $? -ne 0 ]; then
    RESULT="true"
    cp /etc/udev/rules.d/70-persistent-net.rules /etc/udev/rules.d/70-persistent-net.rules.1
    cp $NAMES_FILE /etc/udev/rules.d/70-persistent-net.rules
  fi
else
  RESULT="true"
  cp $NAMES_FILE /etc/udev/rules.d/70-persistent-net.rules
fi

rm -rf $IFACES_FILE
rm -rf $NAMES_FILE
rm -rf $LIST_ETH
rm -rf $LIST_40G

echo -n $RESULT
