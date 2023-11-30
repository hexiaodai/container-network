package network

import (
	"container-network/fn"
	"container-network/network/bridge"
	"container-network/network/overlay"
	"container-network/store"
	"context"
	"fmt"
)

func New() *Mgr {
	return &Mgr{
		network: fn.Args("network"),
	}
}

type Mgr struct {
	network string
	bridge  *bridge.Bridge
	overlay *overlay.Overlay
}

func (m *Mgr) Start(ctx context.Context) error {
	m.bridge = bridge.New()
	store.Instance.RegisterEvents(m.bridge)

	switch m.network {
	case "overlay":
		m.overlay = overlay.New()
		store.Instance.RegisterEvents(m.overlay)
	case "route":
	default:
		return fmt.Errorf("unknown network type: %v", m.network)
	}
	return nil
}

func (m *Mgr) Cleanup() error {
	if err := m.bridge.Cleanup(); err != nil {
		return err
	}
	return m.overlay.Cleanup()
}
