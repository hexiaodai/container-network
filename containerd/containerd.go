package containerd

import (
	"container-network/fn"
	"container-network/store"
	"context"
	"fmt"
	"os/exec"
)

func New() *Containerd {
	return &Containerd{
		NodeName: fn.Args("node"),
	}
}

type Containerd struct {
	NodeName string
}

func (c *Containerd) Update(ctx context.Context, cluster *store.Cluster) {
	// fmt.Println("updating container")

	node, err := store.Instance.ReadNode(c.NodeName)
	if err != nil {
		fn.Errorf("failed to read node: %s", err)
		return
	}
	activeContailer, err := fn.ActiveContailer()
	if err != nil {
		fn.Errorf("failed to get active container: %s", err)
		return
	}
	for _, container := range node.Containers {
		if _, ok := activeContailer[container.Name]; ok {
			continue
		}
		if err := c.setup(node, container); err != nil {
			fn.Errorf("failed to setup container: %s", err)
			continue
		}
	}
}

func (c *Containerd) setup(node *store.Node, container *store.Container) (err error) {
	cmd := exec.Command("ip", "netns", "add", container.Name)
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
		return fmt.Errorf("failed to create netns. cmdout: %v, error: %v", cmdout, err)
	}

	cmd = exec.Command("ip", "link", "add", container.Veth0, "type", "veth", "peer", "name", container.Veth1)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
		return fmt.Errorf("failed to create veth. cmdout: %v, error: %v", cmdout, err)
	}

	cmd = exec.Command("ip", "link", "set", container.Veth0, "netns", container.Name)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, container.Name); err != nil {
		return fmt.Errorf("failed to move veth to netns. cmdout: %v, error: %v", cmdout, err)
	}

	return nil
}

func (c *Containerd) Cleanup() error {
	node, err := store.Instance.ReadNode(c.NodeName)
	if err != nil {
		return fmt.Errorf("failed to read node: %s", err)
	}

	for _, container := range node.Containers {
		cmd := exec.Command("ip", "netns", "delete", container.Name)
		cmdout, err := cmd.CombinedOutput()
		if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
			fn.Errorf("failed to delete netns. cmdout: %v, error: %v", cmdout, err)
		}
	}
	return nil
}
