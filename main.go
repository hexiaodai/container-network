package main

import (
	"container-network/bridge"
	"container-network/containerd"
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	if err := containerd.Init(); err != nil {
		panic(err)
	}
	wg.Add(1)
	go containerd.Running(ctx, &wg)

	if err := bridge.Init(); err != nil {
		panic(err)
	}
	wg.Add(1)
	go bridge.Running(ctx, &wg)

	sign := make(chan os.Signal, 1)
	signal.Notify(sign, syscall.SIGINT, syscall.SIGTERM)
	<-sign
	cancel()

	wg.Wait()
}
