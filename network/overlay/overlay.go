package overlay

import (
	"container-network/fn"
	"container-network/store"
	"context"
	"fmt"
	"os/exec"
)

func New() *Overlay {
	o := &Overlay{NodeName: fn.Args("node"), vxlan100: "vxlan100", dstport: "4789"}
	if err := o.init(); err != nil {
		panic(err)
	}
	return o
}

type Overlay struct {
	NodeName string
	vxlan100 string
	dstport  string
}

func (o *Overlay) init() error {
	node, err := store.Instance.ReadNode(o.NodeName)
	if err != nil {
		return fmt.Errorf("failed to read node: %v", err)
	}

	cmd := exec.Command("ip", "link", "add", o.vxlan100, "type", "vxlan", "id", "100", "local", node.IP, "dev", node.Inertface, "dstport", o.dstport, "nolearning")
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
		return fmt.Errorf("failed to create vxlan100. cmdout: %v. error: %v", cmdout, err)
	}

	cmd = exec.Command("ip", "addr", "add", node.VXLAN.IP, "dev", o.vxlan100)
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
		return fmt.Errorf("failed to add IP to vxlan100. IP: %v. cmdout: %v. error: %v", node.VXLAN.IP, cmdout, err)
	}

	cmd = exec.Command("ip", "link", "set", o.vxlan100, "up")
	cmdout, err = cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to set vxlan100 up. cmdout: %v. error: %v", cmdout, err)
	}

	return nil
}

func (o *Overlay) Update(ctx context.Context, cluster *store.Cluster) {
	fmt.Println("updating overlay")

	for _, node := range cluster.Node {
		if node.Name == o.NodeName {
			continue
		}
		cmd := exec.Command("ip", "route", "add", node.CIDR, "dev", o.vxlan100)
		cmdout, err := cmd.CombinedOutput()
		if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
			fn.Errorf("failed to add CIDR to vxlan100. node: %v. cmdout: %v. error: %v", node, cmdout, err)
		}

		for _, container := range node.Containers {
			cmd = exec.Command("ip", "neighbor", "add", container.IP, "lladdr", node.VXLAN.MAC, "dev", o.vxlan100)
			cmdout, err = cmd.CombinedOutput()
			if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
				fn.Errorf("failed to add container to vxlan100. node: %v. container: %v. cmdout: %v. error: %v", node, container, cmdout, err)
			}

			cmd = exec.Command("bridge", "fdb", "append", node.VXLAN.MAC, "dev", o.vxlan100, "dst", node.IP)
			cmdout, err = cmd.CombinedOutput()
			if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
				fn.Errorf("failed to add container to vxlan100. node: %v. container: %v. cmdout: %v. error: %v", node, container, cmdout, err)
			}
		}
	}
}

func (o *Overlay) Cleanup() error {
	cmd := exec.Command("ip", "link", "del", o.vxlan100)
	cmdout, err := cmd.CombinedOutput()
	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
		return fmt.Errorf("failed to delete vxlan100. cmdout: %v. error: %v", cmdout, err)
	}

	return nil
}

// f6:35:84:38:60:f1
// ip neighbor add 172.18.20.2 lladdr 16:8f:3f:90:b9:2e dev vxlan100
// bridge fdb append 16:8f:3f:90:b9:2e dev vxlan100 dst 192.168.245.172

// sudo ip link add vxlan100 type vxlan \
//     id 100 \
//     local 192.168.245.168 \
//     dev ens33 \
//     dstport 4789 \
//     nolearning

// 16:8f:3f:90:b9:2e
// ip neighbor add 172.18.10.2 lladdr f6:35:84:38:60:f1 dev vxlan100
// bridge fdb append f6:35:84:38:60:f1 dev vxlan100 dst 192.168.245.168

// 	sudo ip link add vxlan100 type vxlan \
//     id 100 \
//     local 192.168.245.172 \
//     dev ens33 \
//     dstport 4789 \
//     nolearning
