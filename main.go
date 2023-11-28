package main

import (
	"fmt"
	"os/exec"
)

func main() {

}

type Container struct {
	Name  string
	Veth0 string
	Veth1 string
}

func NewContainer(containerName string) (container *Container, err error) {
	cmd := exec.Command("ip", "netns", "add", containerName)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	veth0 := fmt.Sprintf("veth-%v-0", containerName)
	veth1 := fmt.Sprintf("veth-%v-1", containerName)

	cmd = exec.Command("ip", "link", "add", veth0, "type", "veth", "peer", "name", veth1)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return
	}

	cmd = exec.Command("ip", "link", "set", veth0, "netns", containerName)
	_, err = cmd.CombinedOutput()

	container = &Container{
		Name:  containerName,
		Veth0: veth0,
		Veth1: veth1,
	}
	return
}
