package main

import (
	"container-network/bridge"
	"fmt"
	"os/exec"
)

func main() {
	container1 := "container1"
	veth0, veth1, err := NewContainer(container1)
	if err != nil {
		panic(err)
	}
	container1IP, container1Port, err := bridge.Setup(container1, veth0, veth1)
	if err != nil {
		panic(err)
	}

	container2 := "container2"
	veth0, veth1, err = NewContainer(container2)
	if err != nil {
		panic(err)
	}
	container2IP, container2Port, err := bridge.Setup(container2, veth0, veth1)
	if err != nil {
		panic(err)
	}

	fmt.Printf("container1: %v. IP: %v. listening port: %v", container1, container1IP, container1Port)
	fmt.Printf("container2: %v. IP: %v. listening port: %v", container2, container2IP, container2Port)

	bridge.Cleanup()
}

func NewContainer(containerName string) (veth0, veth1 string, err error) {
	cmd := exec.Command("ip", "netns", "add", containerName)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	veth0 = fmt.Sprintf("veth-%v-0", containerName)
	veth1 = fmt.Sprintf("veth-%v-1", containerName)

	cmd = exec.Command("ip", "link", "add", veth0, "type", "veth", "peer", "name", veth1)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "link", "set", veth0, "netns", containerName)
	_, err = cmd.CombinedOutput()

	return
}
