package ipam

import (
	"container-network/cluster"
	"container-network/containerd"
	"fmt"
	"net"
	"sync"
)

var locker sync.Locker = &sync.Mutex{}

func FindAvailableIP() (string, error) {
	locker.Lock()
	defer locker.Unlock()

	_, ipNet, err := net.ParseCIDR(cluster.Instance.Current.Container.CIDR)
	if err != nil {
		return "", err
	}

	usedIP := map[string]struct{}{
		cluster.Instance.Current.VXLAN.IP:          {},
		cluster.Instance.Current.Container.Gateway: {},
	}

	// _, ipv4Net, _ := net.ParseCIDR(cluster.Instance.Current.VXLAN.IP)
	// for ip := ipv4Net.IP; ipv4Net.Contains(ip); incrementIP(ip) {
	// 	usedIP[ip.String()] = struct{}{}
	// }

	for _, container := range containerd.Instance.Containers {
		usedIP[container.IP] = struct{}{}
	}

	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		ipStr := ip.String()
		_, ok := usedIP[ipStr]
		if !ok {
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
