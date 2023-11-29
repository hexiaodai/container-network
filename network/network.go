package network

import (
	"container-network/fn"
	"container-network/network/bridge"
	"container-network/store"
	"context"
)

type Network interface {
	Update(ctx context.Context, cluster *store.Cluster)
	Cleanup() error
}

func New() *Mgr {
	return &Mgr{}
}

type Mgr struct {
	network Network
}

func (m *Mgr) Running(ctx context.Context) {
	switch fn.Args("network") {
	case "overlay":
	case "route":
	default:
		m.network = bridge.New()
	}
	store.Instance.RegisterEvents(m.network)
}

func (m *Mgr) Cleanup() error {
	return m.network.Cleanup()
}
