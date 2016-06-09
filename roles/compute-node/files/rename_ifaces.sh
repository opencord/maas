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
	mlx4_en)
            RESULT="MLX4_EN"
            ;;
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
    # this will not support more than 10 fabric nics... should be ok. (Famous last words)
    for i in $(cat $1 | sort); do
        echo "SUBSYSTEM==\"net\", ACTION==\"add\", DRIVERS==\"?*\", ATTR{address}==\"$i\", ATTR{dev_id}==\"0x$IDX\", ATTR{type}==\"1\", KERNEL==\"*\", NAME=\"eth$IDX\"" >> $OUT
        IDX=$(expr $IDX + 1)
    done


    for i in $(cat $2 | sort); do
        echo "SUBSYSTEM==\"net\", ACTION==\"add\", DRIVERS==\"?*\", ATTR{address}==\"$i\", NAME=\"eth$IDX\"" >> $OUT
        IDX=$(expr $IDX + 1)
    done
}

# 40G_LIST ETH_LIST FAB_IFACE EXT_IFACE MGT_IFACE
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
    for i in $(cat $1); do
        echo "auto eth$IDX" >> $OUT
        echo "iface eth$IDX inet manual" >> $OUT
        echo "    bond-master $3" >> $OUT
        [ -z $FIRST ] && echo "    bond-primary eth$IDX" >> $OUT
        # Make bond-mode configurable
        echo "    bond-mode active-backup" >> $OUT
        echo "    bond-miimon 100" >> $OUT
        echo "    bond-slaves none" >> $OUT
        FIRST="done"
        echo "" >> $OUT
        IDX=$(expr $IDX + 1)
    done

    echo "auto $3" >> $OUT
    echo "iface $3 inet static" >> $OUT
    echo "  address $FAB_IP" >> $OUT
    echo "  network $FAB_NETWORK" >> $OUT
    echo "  netmask $FAB_NETMASK" >> $OUT
    # Make bond-mode configurable
    echo "   bond-mode active-backup" >> $OUT
    echo "   bond-miimon 100" >> $OUT
    echo "   bond-slaves none" >> $OUT
    echo "" >> $OUT

    for i in $(cat $2); do
        if [ "eth$IDX" == "$4" ]; then
            if [ "$EXT_IP" == "dhcp" ]; then
		echo "auto eth$IDX" >> $OUT
                echo "iface eth$IDX inet dhcp" >> $OUT
            elif [ "$EXT_IP" == "manual" ]; then
		echo "iface eth$IDX inet manual" >> $OUT
            else
		echo "auto eth$IDX" >> $OUT
                echo "iface eth$IDX inet static" >> $OUT
                echo "    address $EXT_IP" >> $OUT
                echo "    network $EXT_NETWORK" >> $OUT
                echo "    netmask $EXT_NETMASK" >> $OUT
                echo "    broadcast $EXT_BROADCAST" >> $OUT
                echo "    gateway $EXT_GW" >> $OUT
		echo "    dns-nameservers 8.8.8.8 8.8.4.4" >> $OUT
		echo "    dns-search cord.lab" >> $OUT
            fi
        elif [ "eth$IDX" == "$5" ]; then
            echo "auto eth$IDX" >> $OUT
            echo "iface eth$IDX inet dhcp" >> $OUT
        else
            echo "iface eth$IDX inet manual" >> $OUT
        fi
        echo "" >> $OUT
        IDX=$(expr $IDX + 1)
    done
}

FAB_IFACE=$1
FAB_ADDR=$2
FAB_IP=$(echo $FAB_ADDR | cut -d/ -f1)
FAB_MASKBITS=$(echo $FAB_ADDR | cut -d/ -f2)
FAB_NETWORK=$(network $FAB_IP $FAB_MASKBITS)
FAB_NETMASK=$(netmask $FAB_MASKBITS)

EXT_IFACE=$3
EXT_ADDR=$4
if [ "$EXT_ADDR" != "dhcp" ]; then
    EXT_IP=$(echo $EXT_ADDR | cut -d/ -f1)
    EXT_MASKBITS=$(echo $EXT_ADDR | cut -d/ -f2)
    EXT_NETWORK=$(network $EXT_IP $EXT_MASKBITS)
    EXT_NETMASK=$(netmask $EXT_MASKBITS)
    EXT_BROADCAST=$(broadcast $EXT_IP $EXT_MASKBITS)
    EXT_GW=$(first $EXT_ADDR)
fi
MGT_IFACE=$5

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
        I40G|MLX4_EN)
            echo "$(get_mac $i)" >> $LIST_40G
            ;;
        *) ;;
    esac
done

CHANGED="false"

generate_interfaces $LIST_40G $LIST_ETH $FAB_IFACE $EXT_IFACE $MGT_IFACE

diff /etc/network/interfaces $IFACES_FILE 2>&1 > /dev/null
if [ $? -ne 0 ]; then
  CHANGED="true"
  cp /etc/network/interfaces /etc/network/interfaces.1
  cp $IFACES_FILE /etc/network/interfaces
fi

generate_persistent_names $LIST_40G $LIST_ETH $FAB_IFACE $EXT_IFACE
if [ -r /etc/udev/rules.d/70-persistent-net.rules ]; then
  diff /etc/udev/rules.d/70-persistent-net.rules $NAMES_FILE 2>&1 > /dev/null
  if [ $? -ne 0 ]; then
    CHANGED="true"
    cp /etc/udev/rules.d/70-persistent-net.rules /etc/udev/rules.d/70-persistent-net.rules.1
    cp $NAMES_FILE /etc/udev/rules.d/70-persistent-net.rules
  fi
else
  CHANGED="true"
  cp $NAMES_FILE /etc/udev/rules.d/70-persistent-net.rules
fi

rm -rf $IFACES_FILE
rm -rf $NAMES_FILE
rm -rf $LIST_ETH
rm -rf $LIST_40G

echo -n $CHANGED
