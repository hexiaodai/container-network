package main

import (
	"container-network/bridge"
	"container-network/container"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := container.Init(); err != nil {
		panic(err)
	}
	go container.Running()

	if err := bridge.Init(); err != nil {
		panic(err)
	}
	go bridge.Running()

	sign := make(chan os.Signal, 1)
	signal.Notify(sign, syscall.SIGINT, syscall.SIGTERM)
	<-sign
}
