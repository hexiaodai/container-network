package fn

import (
	"fmt"
	"net"
)

func FindAvailableIP(usedIPs []string, cidr string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}

	usedIPMap := make(map[string]bool)
	for _, ip := range usedIPs {
		usedIPMap[ip] = true
	}

	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		ipStr := ip.String()
		if !usedIPMap[ipStr] {
			return ipStr, nil
		}
	}

	return "", fmt.Errorf("no available IP found in CIDR")
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
