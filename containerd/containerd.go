package containerd

import (
	"container-network/fn"
	"container-network/store"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

var (
	nodeName = ""
)

func Init() error {
	nodeName = fn.Args("node")
	if len(nodeName) == 0 {
		return errors.New("failed to parse args. arg: node")
	}
	return nil
}

func Running(ctx context.Context, wg *sync.WaitGroup) {
	for {
		select {
		case <-ctx.Done():
			if err := cleanup(); err != nil {
				fn.Errorf("error cleaning up container: %v", err)
			}
			wg.Done()
			return
		default:
			time.Sleep(time.Second * 5)
		}

		node, err := store.Instance().ReadNode(nodeName)
		if err != nil {
			fn.Errorf("failed to read node: %s", err)
			continue
		}

		// activeContailer, err := f.ActiveContailer()
		// if err != nil {
		// 	f.Errorf("failed to get active container: %s", err)
		// 	continue
		// }
		for _, container := range node.Containers {
			// if _, ok := activeContailer[container.Name]; ok {
			// 	continue
			// }
			if err := setup(node, container); err != nil {
				fn.Errorf("failed to setup container: %s", err)
				continue
			}
		}
	}
}

func setup(node *store.Node, container *store.Container) (err error) {
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

func cleanup() error {
	node, err := store.Instance().ReadNode(nodeName)
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
