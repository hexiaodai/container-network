package overlay

import (
	"container-network/cluster"
	"container-network/fn"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"time"
)

func New() *Overlay {
	o := &Overlay{vxlan100: "vxlan100", dstport: "4789"}
	if err := o.init(); err != nil {
		panic(err)
	}
	return o
}

type Overlay struct {
	vxlan100 string
	dstport  string
}

func (o *Overlay) init() error {
	cmd := exec.Command("ip", "link", "add", o.vxlan100, "type", "vxlan", "id", "100", "local", cluster.Instance.Current.IP, "dev", cluster.Instance.Current.Interface, "dstport", o.dstport, "nolearning")
	cmdout, err := cmd.CombinedOutput()
	if err != nil && !fn.MatchCMDOut(cmdout, "File exists") {
		return fmt.Errorf("failed to create vxlan100. cmdout: %s. error: %v", cmdout, err)
	}

	cmd = exec.Command("ip", "addr", "add", cluster.Instance.Current.VXLAN.IP, "dev", o.vxlan100)
	cmdout, err = cmd.CombinedOutput()
	if err != nil && !fn.MatchCMDOut(cmdout, "File exists") {
		return fmt.Errorf("failed to add IP to vxlan100. IP: %v. cmdout: %s. error: %v", cluster.Instance.Current.VXLAN.IP, cmdout, err)
	}

	cmd = exec.Command("ip", "link", "set", o.vxlan100, "up")
	cmdout, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set vxlan100 up. cmdout: %s. error: %v", cmdout, err)
	}

	cmd = exec.Command("ip", "link", "show", o.vxlan100)
	cmdout, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to show vxlan100. cmdout: %s. error: %v", cmdout, err)
	}
	re := regexp.MustCompile(`link/ether\s([0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5})`)
	matches := re.FindStringSubmatch(string(cmdout))
	if len(matches) < 2 {
		return fmt.Errorf("no MAC address found for vxlan100. cmdout: %s", cmdout)
	}
	cluster.Instance.Current.VXLAN.MAC = matches[1]
	return nil
}

func (o *Overlay) Running(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 5):
			for _, node := range cluster.Instance.Nodes {
				cmd := exec.Command("ip", "route", "add", node.Container.CIDR, "dev", o.vxlan100)
				cmdout, err := cmd.CombinedOutput()
				if err != nil && !fn.MatchCMDOut(cmdout, "File exists") {
					fn.Errorf("failed to add CIDR to vxlan100. node: %v. cmdout: %s. error: %v", node, cmdout, err)
				}
				containers, err := cluster.Instance.GetContainers(ctx, node.IP)
				if err != nil {
					fn.Errorf("failed to get containers. node: %v. error: %v", node, err)
					continue
				}
				mac, err := cluster.Instance.GetVXLANMAC(ctx, node.IP)
				if err != nil {
					fn.Errorf("failed to get vxlan mac. node: %v. error: %v", node, err)
					continue
				}
				for _, container := range containers {
					if len(container.IP) == 0 {
						continue
					}
					cmd = exec.Command("ip", "neighbor", "add", container.IP, "lladdr", mac, "dev", o.vxlan100)
					cmdout, err = cmd.CombinedOutput()
					if err != nil && !fn.MatchCMDOut(cmdout, "File exists") {
						fn.Errorf("failed to add container to vxlan100. node: %+v. VXLAN: %+v. container: %+v. cmdout: %s. error: %v", node, node.VXLAN, container, cmdout, err)
					}

					cmd = exec.Command("bridge", "fdb", "append", mac, "dev", o.vxlan100, "dst", node.IP)
					cmdout, err = cmd.CombinedOutput()
					if err != nil {
						fn.Errorf("failed to add container to vxlan100. node: %+v. VXLAN: %+v. container: %+v. cmdout: %s. error: %v", node, node.VXLAN, container, cmdout, err)
					}
				}
			}
		}
	}
}

// func (o *Overlay) Update(ctx context.Context, cluster *store.Cluster) {
// 	// fmt.Println("updating overlay")

// 	for _, node := range cluster.Node {
// 		if node.Name == o.NodeName {
// 			continue
// 		}
// 		cmd := exec.Command("ip", "route", "add", node.CIDR, "dev", o.vxlan100)
// 		cmdout, err := cmd.CombinedOutput()
// 		if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
// 			fn.Errorf("failed to add CIDR to vxlan100. node: %v. cmdout: %s. error: %v", node, cmdout, err)
// 		}

// 		for _, container := range node.Containers {
// 			cmd = exec.Command("ip", "neighbor", "add", container.IP, "lladdr", node.VXLAN.MAC, "dev", o.vxlan100)
// 			cmdout, err = cmd.CombinedOutput()
// 			if cmdout, err := fn.CheckCMDOut(cmdout, err, "File exists"); err != nil {
// 				fn.Errorf("failed to add container to vxlan100. node: %+v. VXLAN: %+v. container: %+v. cmdout: %s. error: %v", node, node.VXLAN, container, cmdout, err)
// 			}

// 			cmd = exec.Command("bridge", "fdb", "append", node.VXLAN.MAC, "dev", o.vxlan100, "dst", node.IP)
// 			cmdout, err = cmd.CombinedOutput()
// 			if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
// 				fn.Errorf("failed to add container to vxlan100. node: %+v. VXLAN: %+v. container: %+v. cmdout: %s. error: %v", node, node.VXLAN, container, cmdout, err)
// 			}
// 		}
// 	}
// }

// func (o *Overlay) Cleanup() error {
// 	cmd := exec.Command("ip", "link", "del", o.vxlan100)
// 	cmdout, err := cmd.CombinedOutput()
// 	if cmdout, err := fn.CheckCMDOut(cmdout, err); err != nil {
// 		return fmt.Errorf("failed to delete vxlan100. cmdout: %s. error: %v", cmdout, err)
// 	}

// 	return nil
// }

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
