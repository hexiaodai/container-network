package bridge

import (
	"fmt"
	"os/exec"
)

var (
	ipCount = 0

	br = "br0"

	brVeths = []string{}
)

func init() {
	cmd := exec.Command("sysctl", "net.ipv4.conf.all.forwarding=1")
	_, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}

	cmd = exec.Command("iptables", "-P", "FORWARD", "ACCEPT")
	_, err = cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}

	cmd = exec.Command("iptables", "-t", "-A", "POSTROUTING", "-s", "172.18.0.0/24", "!", "-o", br, "-j", "MASQUERADE")
	_, err = cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
}

func Setup(containerName, veth0, veth1 string) (containerIP, port string, err error) {
	containerIP = fmt.Sprintf("172.18.0.%v", ipCount+2)

	cmd := exec.Command("brctl", "addbr", br)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("brctl", "addif", br, veth0)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "netns", "exec", containerName, "ip", "addr", "add", fmt.Sprintf("%v/24", containerIP), "dev", veth0)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "netns", "exec", containerName, "ip", "link", "set", veth0, "up")
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "link", "set", veth1, "up")
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "addr", "add", "172.18.0.1/24", "dev", br)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "link", "set", br, "up")
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "netns", "exec", containerName, "route", "add", "default", "gw", "172.18.0.1", veth0)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	port = fmt.Sprintf("8%v", ipCount)
	cmd = exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "!", "-i", br, "-p", "tcp", "-m", "tcp", "--dport", port, "-j", "DNAT", "--to-destination", fmt.Sprintf("%v:%v", containerIP, port))
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "netns", "exec", containerName, "nc", "-lp", port)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	brVeths = append(brVeths, veth1)
	ipCount++
	return
}

func Cleanup() (err error) {
	cmd := exec.Command("ip", "link", "set", br, "down")
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("brctl", "delbr", br)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	for _, veth := range brVeths {
		cmd = exec.Command("ip", "link", "del", veth)
		_, err = cmd.CombinedOutput()
		if err != nil {
			return
		}
	}

	return
}
