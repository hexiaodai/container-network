package container

import (
	"container-network/f"
	"container-network/store"
	"errors"
	"os/exec"
	"time"
)

var (
	nodeName = ""
)

func Init() error {
	nodeName = f.Args("node")
	if len(nodeName) == 0 {
		return errors.New("failed to parse args. arg: node")
	}
	return nil
}

func Running() {
	for {
		node, err := store.Instance().ReadNode(nodeName)
		if err != nil {
			f.Errorf("failed to read node: %s", err)
			continue
		}

		activeContailer, err := f.ActiveContailer()
		if err != nil {
			f.Errorf("failed to get active container: %s", err)
			continue
		}
		for _, container := range node.Containers {
			if _, ok := activeContailer[container.Name]; ok {
				continue
			}
			setup(node, container)
		}
		time.Sleep(time.Second * 3)
	}
}

func setup(node *store.Node, container *store.Container) (err error) {
	cmd := exec.Command("ip", "netns", "add", container.Name)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "link", "add", container.Veth0, "type", "veth", "peer", "name", container.Veth1)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "link", "set", container.Veth0, "netns", container.Name)
	_, err = cmd.CombinedOutput()

	return
}
