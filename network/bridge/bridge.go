package bridge

import (
	"container-network/cluster"
	"container-network/containerd"
	"container-network/fn"
	"container-network/network/ipam"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func New() *Bridge {
	b := &Bridge{Br0: "br0"}
	if err := b.init(); err != nil {
		panic(err)
	}
	return b
}

type Bridge struct {
	Br0 string
}

func (b *Bridge) setVethPairs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 5):
			for _, container := range containerd.Instance.List() {
				if len(container.Veth0) != 0 && len(container.Veth1) != 0 {
					continue
				}
				veth0 := fmt.Sprintf("veth0%v", container.Name)
				veth1 := fmt.Sprintf("veth1%v", container.Name)

				cmd := exec.Command("ip", "link", "add", veth0, "type", "veth", "peer", "name", veth1)
				cmdout, err := cmd.CombinedOutput()
				if err != nil && !fn.MatchCMDOut(cmdout, "File exists") {
					fn.Errorf("failed to create veth pair. cmdout: %s, error: %v", cmdout, err)
					continue
				}

				cmd = exec.Command("ip", "link", "set", veth0, "netns", container.Name)
				cmdout, err = cmd.CombinedOutput()
				if err != nil {
					fn.Errorf("failed to set veth0 to netns. cmdout: %s, error: %v", cmdout, err)
					continue
				}
				newContainer := container
				newContainer.Veth0 = veth0
				newContainer.Veth1 = veth1
				containerd.Instance.Set(newContainer)
			}
		}
	}
}

func (b *Bridge) init() error {
	cmd := exec.Command("brctl", "addbr", b.Br0)
	cmdout, err := cmd.CombinedOutput()
	if err != nil && !fn.MatchCMDOut(cmdout, "already exists") {
		return fmt.Errorf("failed to create bridge. cmdout: %s. error: %v", cmdout, err)
	}

	cmd = exec.Command("ip", "addr", "add", fmt.Sprintf("%v/24", cluster.Instance.Current.Container.Gateway), "dev", b.Br0)
	cmdout, err = cmd.CombinedOutput()
	if err != nil && !fn.MatchCMDOut(cmdout, "File exists") {
		return fmt.Errorf("failed to add ip to bridge. cmdout: %s. error: %v", cmdout, err)
	}

	cmd = exec.Command("ip", "link", "set", b.Br0, "up")
	cmdout, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to bring up bridge. cmdout: %s. error: %v", cmdout, err)
	}

	cmd = exec.Command("sysctl", "net.ipv4.conf.all.forwarding=1")
	cmdout, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set net.ipv4.conf.all.forwarding=1: %s. cmdout: %s", err, cmdout)
	}

	matched, err := b.matchedPOSTROUTING(cluster.Instance.Current)
	if err != nil {
		return fmt.Errorf("failed to match POSTROUTING: %v", err)
	}
	if !matched {
		cmd = exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", cluster.Instance.Current.Container.CIDR, "!", "-o", b.Br0, "-j", "MASQUERADE")
		cmdout, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set POSTROUTING: %s. cmdout: %s", err, cmdout)
		}
	}

	b.initContainers()

	return nil
}

func (b *Bridge) initContainers() {
	for _, container := range containerd.Instance.List() {
		newContainer := container
		cmd := exec.Command("ip", "netns", "exec", container.Name, "ip", "addr", "show", "veth0"+container.Name)
		// cmd := exec.Command("ip", "netns", "exec", container.Name, "ip", "link", "show", "veth0"+container.Name)
		if cmdout, err := cmd.CombinedOutput(); err == nil {
			re := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
			newContainer.Veth0 = fmt.Sprintf("veth0%v", container.Name)
			newContainer.Veth1 = fmt.Sprintf("veth1%v", container.Name)
			newContainer.IP = re.FindString(string(cmdout))
			containerd.Instance.Set(newContainer)
		}
	}
}

func (b *Bridge) Running(ctx context.Context) {
	go b.setVethPairs(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 5):
			for _, container := range containerd.Instance.List() {
				if len(container.IP) > 0 || len(container.Veth0) == 0 || len(container.Veth1) == 0 {
					continue
				}
				containerIP, err := b.setup(container)
				if err != nil {
					fn.Errorf("failed to setup veth pair for container %s: %v", container.Name, err)
					continue
				}
				newContainer := container
				newContainer.IP = containerIP
				containerd.Instance.Set(newContainer)
			}
		}
	}
}

func (b *Bridge) setup(container *containerd.Container) (containerIP string, err error) {
	containerIP, err = ipam.FindAvailableIP()
	if err != nil {
		return containerIP, fmt.Errorf("failed to find available ip: %v", err)
	}
	cmd := exec.Command("brctl", "addif", b.Br0, container.Veth1)
	cmdout, err := cmd.CombinedOutput()
	if err != nil && !fn.MatchCMDOut(cmdout, "already") {
		return containerIP, fmt.Errorf("failed to add veth to bridge. container: %+v. cmdout: %s. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "ip", "addr", "add", fmt.Sprintf("%v/24", containerIP), "dev", container.Veth0)
	cmdout, err = cmd.CombinedOutput()
	if err != nil && !fn.MatchCMDOut(cmdout, "File exists") {
		return containerIP, fmt.Errorf("failed to add ip to veth. container: %+v. cmdout: %s. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "ip", "link", "set", container.Veth0, "up")
	cmdout, err = cmd.CombinedOutput()
	if err != nil {
		return containerIP, fmt.Errorf("failed to bring up veth. container: %+v. cmdout: %s. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "link", "set", container.Veth1, "up")
	cmdout, err = cmd.CombinedOutput()
	if err != nil {
		return containerIP, fmt.Errorf("failed to bring up veth. container: %+v. cmdout: %s. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "route", "add", "default", "gw", cluster.Instance.Current.Container.Gateway, container.Veth0)
	cmdout, err = cmd.CombinedOutput()
	if err != nil && !fn.MatchCMDOut(cmdout, "File exists") {
		return containerIP, fmt.Errorf("failed to add default route. container: %+v. cmdout: %s. error: %v", container, cmdout, err)
	}

	// matched, err := b.matchedPREROUTING(container)
	// if err != nil {
	// 	return fmt.Errorf("failed to check PREROUTING. container: %+v. error: %v", container, err)
	// }
	// if !matched {
	// 	cmd = exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "!", "-i", b.Br0, "-p", "tcp", "-m", "tcp", "--dport", fmt.Sprintf("%v", container.ContainerPort), "-j", "DNAT", "--to-destination", fmt.Sprintf("%v:%v", container.IP, container.HostPort))
	// 	cmdout, err = cmd.CombinedOutput()
	// 	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
	// 		return fmt.Errorf("failed to add iptables rule. container: %+v. cmdout: %s. error: %v", container, cmdout, err)
	// 	}
	// }

	return containerIP, nil
}

// func (b *Bridge) matchedPREROUTING(container *containerd.Container) (bool, error) {
// 	cmd := exec.Command("iptables", "-t", "nat", "-S", "PREROUTING")
// 	cmdout, err := cmd.CombinedOutput()
// 	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
// 		return false, fmt.Errorf("failed to get iptables rules. container: %+v. cmdout: %s. error: %v", container, cmdout, err)
// 	}
// 	rule := fmt.Sprintf("! -i %v -p tcp -m tcp --dport %v -j DNAT --to-destination %v:%v", b.Br0, container.ContainerPort, container.IP, container.HostPort)
// 	rules := strings.Split(string(cmdout), "\n")
// 	matched := false
// 	for _, r := range rules {
// 		if strings.Contains(r, rule) {
// 			matched = true
// 		}
// 	}
// 	return matched, nil
// }

func (b *Bridge) matchedPOSTROUTING(node *cluster.Node) (bool, error) {
	cmd := exec.Command("iptables", "-t", "nat", "-S", "POSTROUTING")
	cmdout, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to get iptables rules. node: %+v. cmdout: %s. error: %v", node, cmdout, err)
	}
	rule := fmt.Sprintf("-s %v ! -o %v -j MASQUERADE", node.Container.CIDR, b.Br0)
	rules := strings.Split(string(cmdout), "\n")
	matched := false
	for _, r := range rules {
		if strings.Contains(r, rule) {
			matched = true
		}
	}
	return matched, nil
}

// func (b *Bridge) Cleanup() error {
// 	cmd := exec.Command("ip", "link", "set", b.Br0, "down")
// 	cmdout, err := cmd.CombinedOutput()
// 	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
// 		return fmt.Errorf("failed to set down br0. cmdout: %s. error: %v", cmdout, err)
// 	}

// 	cmd = exec.Command("brctl", "delbr", b.Br0)
// 	cmdout, err = cmd.CombinedOutput()
// 	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
// 		return fmt.Errorf("failed to delete br0. cmdout: %s. error: %v", cmdout, err)
// 	}

// 	node, err := store.Instance.ReadNode(b.NodeName)
// 	if err != nil {
// 		return fmt.Errorf("failed to read node. node: %v. error: %v", b.NodeName, err)
// 	}

// 	for _, container := range node.Containers {
// 		cmd = exec.Command("ip", "link", "del", container.Veth1)
// 		cmdout, err = cmd.CombinedOutput()
// 		if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
// 			fn.Errorf("failed to delete veth1. container: %+v. cmdout: %s. error: %v", container, cmdout, err)
// 			continue
// 		}
// 	}

// 	return nil
// }
