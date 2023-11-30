package bridge

import (
	"container-network/fn"
	"container-network/store"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func New() *Bridge {
	b := &Bridge{NodeName: fn.Args("node"), Br0: "br0"}
	if err := b.init(); err != nil {
		panic(err)
	}
	return b
}

type Bridge struct {
	NodeName string
	Br0      string
}

func (b *Bridge) init() error {
	node, err := store.Instance.ReadNode(b.NodeName)
	if err != nil {
		return fmt.Errorf("failed to read node: %v", err)
	}

	cmd := exec.Command("brctl", "addbr", b.Br0)
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "already exists"); err != nil {
		return fmt.Errorf("failed to create bridge. cmdout: %v. error: %v", cmdout, err)
	}

	cmd = exec.Command("ip", "addr", "add", fmt.Sprintf("%v/24", node.Gateway), "dev", b.Br0)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
		return fmt.Errorf("failed to add ip to bridge. cmdout: %v. error: %v", cmdout, err)
	}

	cmd = exec.Command("ip", "link", "set", b.Br0, "up")
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to bring up bridge. cmdout: %v. error: %v", cmdout, err)
	}

	cmd = exec.Command("sysctl", "net.ipv4.conf.all.forwarding=1")
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to set net.ipv4.conf.all.forwarding=1: %s. cmdout: %v", err, cmdout)
	}

	matched, err := b.matchedPOSTROUTING(node)
	if err != nil {
		return fmt.Errorf("failed to match POSTROUTING: %v", err)
	}
	if !matched {
		cmd = exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", node.CIDR, "!", "-o", b.Br0, "-j", "MASQUERADE")
		cmdout, err := cmd.CombinedOutput()
		if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
			return fmt.Errorf("failed to set POSTROUTING: %s. cmdout: %v", err, cmdout)
		}
	}

	return nil
}

func (b *Bridge) Update(ctx context.Context, cluster *store.Cluster) {
	fmt.Println("updating bridge")

	node, err := store.Instance.ReadNode(b.NodeName)
	if err != nil {
		fn.Errorf("failed to read node: %v", err)
		return
	}
	for _, container := range node.Containers {
		if err := b.setup(node, container); err != nil {
			fn.Errorf("failed to setup bridge: %v", err)
			continue
		}
	}
}

func (b *Bridge) setup(node *store.Node, container *store.Container) (err error) {
	cmd := exec.Command("brctl", "addif", b.Br0, container.Veth1)
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "already"); err != nil {
		return fmt.Errorf("failed to add veth to bridge. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "ip", "addr", "add", fmt.Sprintf("%v/24", container.IP), "dev", container.Veth0)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
		return fmt.Errorf("failed to add ip to veth. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "ip", "link", "set", container.Veth0, "up")
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to bring up veth. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "link", "set", container.Veth1, "up")
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to bring up veth. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "route", "add", "default", "gw", node.Gateway, container.Veth0)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
		return fmt.Errorf("failed to add default route. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}

	matched, err := b.matchedPREROUTING(container)
	if err != nil {
		return fmt.Errorf("failed to check PREROUTING. container: %+v. error: %v", container, err)
	}
	if !matched {
		cmd = exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "!", "-i", b.Br0, "-p", "tcp", "-m", "tcp", "--dport", fmt.Sprintf("%v", container.ContainerPort), "-j", "DNAT", "--to-destination", fmt.Sprintf("%v:%v", container.IP, container.HostPort))
		cmdout, err = cmd.CombinedOutput()
		if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
			return fmt.Errorf("failed to add iptables rule. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
		}
	}

	return nil
}

func (b *Bridge) matchedPREROUTING(container *store.Container) (bool, error) {
	cmd := exec.Command("iptables", "-t", "nat", "-S", "PREROUTING")
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return false, fmt.Errorf("failed to get iptables rules. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}
	rule := fmt.Sprintf("! -i %v -p tcp -m tcp --dport %v -j DNAT --to-destination %v:%v", b.Br0, container.ContainerPort, container.IP, container.HostPort)
	rules := strings.Split(string(cmdout), "\n")
	matched := false
	for _, r := range rules {
		if strings.Contains(r, rule) {
			matched = true
		}
	}
	return matched, nil
}

func (b *Bridge) matchedPOSTROUTING(node *store.Node) (bool, error) {
	cmd := exec.Command("iptables", "-t", "nat", "-S", "POSTROUTING")
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return false, fmt.Errorf("failed to get iptables rules. node: %+v. cmdout: %v. error: %v", node, cmdout, err)
	}
	rule := fmt.Sprintf("-s %v ! -o %v -j MASQUERADE", node.CIDR, b.Br0)
	rules := strings.Split(string(cmdout), "\n")
	matched := false
	for _, r := range rules {
		if strings.Contains(r, rule) {
			matched = true
		}
	}
	return matched, nil
}

func (b *Bridge) Cleanup() error {
	cmd := exec.Command("ip", "link", "set", b.Br0, "down")
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to set down br0. cmdout: %v. error: %v", cmdout, err)
	}

	cmd = exec.Command("brctl", "delbr", b.Br0)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to delete br0. cmdout: %v. error: %v", cmdout, err)
	}

	node, err := store.Instance.ReadNode(b.NodeName)
	if err != nil {
		return fmt.Errorf("failed to read node. node: %v. error: %v", b.NodeName, err)
	}

	for _, container := range node.Containers {
		cmd = exec.Command("ip", "link", "del", container.Veth1)
		cmdout, err = cmd.CombinedOutput()
		if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
			fn.Errorf("failed to delete veth1. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
			continue
		}
	}

	return nil
}
