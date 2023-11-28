package bridge

import (
	"container-network/f"
	"container-network/store"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

var (
	br0      = "br0"
	nodeName = ""
)

func Init() error {
	cmd := exec.Command("sysctl", "net.ipv4.conf.all.forwarding=1")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	cmd = exec.Command("iptables", "-P", "FORWARD", "ACCEPT")
	_, err = cmd.CombinedOutput()
	if err != nil {
		return err
	}

	nodeName = f.Args("node")
	if len(nodeName) == 0 {
		return errors.New("failed to parse args. arg: node")
	}

	// cmd = exec.Command("iptables", "-t", "-A", "POSTROUTING", "-s", node.CIDR, "!", "-o", br0, "-j", "MASQUERADE")
	// _, err = cmd.CombinedOutput()
	// if err != nil {
	// 	return err
	// }

	return nil
}

func Running() {
	for {
		node, err := store.Instance().ReadNode(nodeName)
		if err != nil {
			f.Errorf("failed to read node: %v", err)
			continue
		}
		for _, container := range node.Containers {
			if err := setup(node, container); err != nil {
				f.Errorf("failed to setup container: %v", err)
				continue
			}
		}
		time.Sleep(time.Second * 3)
	}
}

func setup(node *store.Node, container *store.Container) (err error) {
	cmd := exec.Command("brctl", "addbr", br0)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("brctl", "addif", br0, container.Veth0)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "ip", "addr", "add", fmt.Sprintf("%v/24", container.IP), "dev", container.Veth0)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "ip", "link", "set", container.Veth0, "up")
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "link", "set", container.Veth1, "up")
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "addr", "add", node.CIDR, "dev", br0)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "link", "set", br0, "up")
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "route", "add", "default", "gw", node.Gateway, container.Veth0)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "!", "-i", br0, "-p", "tcp", "-m", "tcp", "--dport", fmt.Sprintf("%v", container.ContainerPort), "-j", "DNAT", "--to-destination", fmt.Sprintf("%v:%v", container.IP, container.HostPort))
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	// cmd = exec.Command("ip", "netns", "exec", container.Name, "nc", "-lp", fmt.Sprintf("%v", container.ContainerPort))
	// _, err = cmd.CombinedOutput()
	// if err != nil {
	// 	return
	// }
	return
}
