package main

import (
	"fmt"
	"strconv"
	"strings"
)

type IPv4 uint32

func ParseIP(dot string) (IPv4, error) {
	var ip IPv4 = 0
	o := strings.Split(dot, ".")
	for i := 0; i < 4; i += 1 {
		b, _ := strconv.Atoi(o[i])
		ip = ip | (IPv4(byte(b)) << (uint(3-i) * 8))
	}
	return ip, nil
}

func (ip IPv4) Next() (IPv4, error) {
	return ip + 1, nil
}

func (ip IPv4) String() string {
	b := []byte{0, 0, 0, 0}
	for i := 0; i < 4; i += 1 {
		m := IPv4(255) << uint((3-i)*8)
		b[i] = byte(((ip & m) >> uint((3-i)*8)))
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
}
