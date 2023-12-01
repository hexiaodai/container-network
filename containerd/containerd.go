package containerd

import (
	"container-network/fn"
	"context"
	"sync"
	"time"
)

var Instance *Containerd = New()

func New() *Containerd {
	names, err := fn.Containers()
	if err != nil {
		panic(err)
	}
	c := &Containerd{
		Containers: map[string]*Container{},
	}
	for _, name := range names {
		if _, ok := c.Containers[name]; !ok {
			c.Containers[name] = &Container{Name: name}
		}
	}
	return c
}

type Containerd struct {
	Containers map[string]*Container
	sync.Mutex
}

func (c *Containerd) Set(container *Container) {
	c.Lock()
	defer c.Unlock()
	c.Containers[container.Name] = container
}

func (c *Containerd) List() map[string]*Container {
	c.Lock()
	defer c.Unlock()
	return c.Containers
}

func (c *Containerd) Get(name string) (*Container, bool) {
	c.Lock()
	defer c.Unlock()
	value, ok := c.Containers[name]
	return value, ok
}

func (c *Containerd) Running(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 5):
			names, err := fn.Containers()
			if err != nil {
				fn.Errorf("failed to get containers: %s", err)
			}
			for _, name := range names {
				if _, ok := c.Containers[name]; !ok {
					c.Containers[name] = &Container{Name: name}
				}
			}
		}
	}
}

type Container struct {
	Name  string
	IP    string
	Veth0 string
	Veth1 string
	// ContainerPort string
	// HostPort      string
}

// func (c *Containerd) Update(ctx context.Context, cluster *store.Cluster) {
// 	// fmt.Println("updating container")

// 	node, err := store.Instance.ReadNode(c.NodeName)
// 	if err != nil {
// 		fn.Errorf("failed to read node: %s", err)
// 		return
// 	}
// 	activeContailer, err := fn.ActiveContailer()
// 	if err != nil {
// 		fn.Errorf("failed to get active container: %s", err)
// 		return
// 	}
// 	for _, container := range node.Containers {
// 		if _, ok := activeContailer[container.Name]; ok {
// 			continue
// 		}
// 		if err := c.setup(node, container); err != nil {
// 			fn.Errorf("failed to setup container: %s", err)
// 			continue
// 		}
// 	}
// }

// func (c *Containerd) setup(node *store.Node, container *store.Container) (err error) {
// 	cmd := exec.Command("ip", "netns", "add", container.Name)
// 	cmdout, err := cmd.CombinedOutput()
// 	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
// 		return fmt.Errorf("failed to create netns. cmdout: %v, error: %v", cmdout, err)
// 	}

// 	cmd = exec.Command("ip", "link", "add", container.Veth0, "type", "veth", "peer", "name", container.Veth1)
// 	cmdout, err = cmd.CombinedOutput()
// 	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
// 		return fmt.Errorf("failed to create veth. cmdout: %v, error: %v", cmdout, err)
// 	}

// 	cmd = exec.Command("ip", "link", "set", container.Veth0, "netns", container.Name)
// 	cmdout, err = cmd.CombinedOutput()
// 	if cmdout, err := fn.CheckCMDOut(cmdout, err, container.Name); err != nil {
// 		return fmt.Errorf("failed to move veth to netns. cmdout: %v, error: %v", cmdout, err)
// 	}

// 	return nil
// }

// func (c *Containerd) Cleanup() error {
// 	node, err := store.Instance.ReadNode(c.NodeName)
// 	if err != nil {
// 		return fmt.Errorf("failed to read node: %s", err)
// 	}

// 	for _, container := range node.Containers {
// 		cmd := exec.Command("ip", "netns", "delete", container.Name)
// 		cmdout, err := cmd.CombinedOutput()
// 		if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
// 			fn.Errorf("failed to delete netns. cmdout: %v, error: %v", cmdout, err)
// 		}
// 	}
// 	return nil
// }
