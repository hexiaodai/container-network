package bridge

import (
	"container-network/fn"
	"container-network/store"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
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

	nodeName = fn.Args("node")
	if len(nodeName) == 0 {
		return errors.New("failed to parse args. arg: node")
	}

	node, err := store.Instance().ReadNode(nodeName)
	if err != nil {
		return fmt.Errorf("failed to read node: %v", err)
	}
	matched, err := matchedPOSTROUTING(node)
	if err != nil {
		return fmt.Errorf("failed to match POSTROUTING: %v", err)
	}
	if !matched {
		cmd = exec.Command("iptables", "-t", "-A", "POSTROUTING", "-s", node.CIDR, "!", "-o", br0, "-j", "MASQUERADE")
		_, err = cmd.CombinedOutput()
		if err != nil {
			return err
		}
	}

	return nil
}

func Running(ctx context.Context, wg *sync.WaitGroup) {
	for {
		select {
		case <-ctx.Done():
			if err := cleanup(); err != nil {
				fn.Errorf("error cleaning up bridge: %v", err)
			}
			wg.Done()
			return
		default:
			time.Sleep(time.Second * 5)
		}

		node, err := store.Instance().ReadNode(nodeName)
		if err != nil {
			fn.Errorf("failed to read node: %v", err)
			continue
		}
		for _, container := range node.Containers {
			if err := setup(node, container); err != nil {
				fn.Errorf("failed to setup bridge: %v", err)
				continue
			}
		}
	}
}

func setup(node *store.Node, container *store.Container) (err error) {
	cmd := exec.Command("brctl", "addbr", br0)
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "already exists"); err != nil {
		return fmt.Errorf("failed to create bridge. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("brctl", "addif", br0, container.Veth1)
	cmdout, err = cmd.CombinedOutput()
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

	cmd = exec.Command("ip", "addr", "add", node.CIDR, "dev", br0)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
		return fmt.Errorf("failed to add ip to bridge. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "link", "set", br0, "up")
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to bring up bridge. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}

	cmd = exec.Command("ip", "netns", "exec", container.Name, "route", "add", "default", "gw", node.Gateway, container.Veth0)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
		return fmt.Errorf("failed to add default route. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}

	matched, err := matchedPREROUTING(container)
	if err != nil {
		return fmt.Errorf("failed to check PREROUTING. container: %+v. error: %v", container, err)
	}
	if !matched {
		cmd = exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "!", "-i", br0, "-p", "tcp", "-m", "tcp", "--dport", fmt.Sprintf("%v", container.ContainerPort), "-j", "DNAT", "--to-destination", fmt.Sprintf("%v:%v", container.IP, container.HostPort))
		cmdout, err = cmd.CombinedOutput()
		if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
			return fmt.Errorf("failed to add iptables rule. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
		}
	}

	// cmd = exec.Command("ip", "netns", "exec", container.Name, "nc", "-lp", fmt.Sprintf("%v", container.ContainerPort))
	// _, err = cmd.CombinedOutput()
	// if err != nil {
	// 	return
	// }
	return nil
}

func matchedPREROUTING(container *store.Container) (bool, error) {
	cmd := exec.Command("iptables", "-t", "nat", "-S", "PREROUTING")
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return false, fmt.Errorf("failed to get iptables rules. container: %+v. cmdout: %v. error: %v", container, cmdout, err)
	}
	rule := fmt.Sprintf("! -i %v -p tcp -m tcp --dport %v -j DNAT --to-destination %v:%v", br0, container.ContainerPort, container.IP, container.HostPort)
	rules := strings.Split(string(cmdout), "\n")
	matched := false
	for _, r := range rules {
		if strings.Contains(r, rule) {
			matched = true
		}
	}
	return matched, nil
}

func matchedPOSTROUTING(node *store.Node) (bool, error) {
	cmd := exec.Command("iptables", "-t", "nat", "-S", "POSTROUTING")
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return false, fmt.Errorf("failed to get iptables rules. node: %+v. cmdout: %v. error: %v", node, cmdout, err)
	}
	rule := fmt.Sprintf("-s %v ! -o %v -j MASQUERADE", node.CIDR, br0)
	rules := strings.Split(string(cmdout), "\n")
	matched := false
	for _, r := range rules {
		if strings.Contains(r, rule) {
			matched = true
		}
	}
	return matched, nil
}

func cleanup() error {
	cmd := exec.Command("ip", "link", "set", br0, "down")
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to set down br0. cmdout: %v. error: %v", cmdout, err)
	}

	cmd = exec.Command("brctl", "delbr", br0)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to delete br0. cmdout: %v. error: %v", cmdout, err)
	}

	node, err := store.Instance().ReadNode(nodeName)
	if err != nil {
		return fmt.Errorf("failed to read node. node: %v. error: %v", nodeName, err)
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
