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

func ParseS3Path(path string) (bucket, key string) {

	if strings.HasPrefix(path, "s3://") {
		firstSlash := strings.Index(path[5:], "/")
		if firstSlash > 0 {
			bucket = path[5 : 5+firstSlash]
			key = path[5+firstSlash+1:]
		} else {
			bucket = path[5:]
		}
	}
	return bucket, key

}
