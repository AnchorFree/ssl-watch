package main

import (
	"net"
	"strings"
)

func IsIPv4(address string) bool {
	return strings.Count(address, ":") < 2
}

func StrToIp(IPList []string) []net.IP {

	ips := []net.IP{}

	for _, ipString := range IPList {
		ip := net.ParseIP(ipString)
		if ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips

}
