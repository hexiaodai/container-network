package main

import (
	"container-network/bridge"
	"container-network/containerd"
	"container-network/fn"
	"container-network/store"
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	containerd := containerd.New()
	store.Instance.RegisterEvents(containerd)

	bridge := bridge.New()
	store.Instance.RegisterEvents(bridge)

	wg.Add(1)
	go store.Instance.Running(ctx, &wg)

	sign := make(chan os.Signal, 1)
	signal.Notify(sign, syscall.SIGINT, syscall.SIGTERM)
	<-sign
	cancel()

	wg.Wait()

	if fn.Args("cleanup") != "false" {
		if err := bridge.Cleanup(); err != nil {
			fn.Errorf("error cleaning up bridge: %v", err)
		}
		if err := containerd.Cleanup(); err != nil {
			fn.Errorf("error cleaning up container: %v", err)
		}
	}
}
