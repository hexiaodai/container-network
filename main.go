package main

import (
	"container-network/cluster"
	"container-network/containerd"
	"container-network/network"
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	go cluster.Instance.Running(ctx)

	go containerd.Instance.Running(ctx)

	go network.New().Running(ctx)

	sign := make(chan os.Signal, 1)
	signal.Notify(sign, syscall.SIGINT, syscall.SIGTERM)
	<-sign
	cancel()
}
