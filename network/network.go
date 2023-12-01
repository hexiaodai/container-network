package network

import (
	"container-network/fn"
	"container-network/network/bridge"
	"container-network/network/overlay"
	"context"
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

func (m *Mgr) Running(ctx context.Context) {
	m.bridge = bridge.New()
	go m.bridge.Running(ctx)

	switch m.network {
	case "overlay":
		m.overlay = overlay.New()
		m.overlay.Running(ctx)
	case "route":
	default:
		// return fmt.Errorf("unknown network type: %v", m.network)
	}
}

// func (m *Mgr) Cleanup() error {
// 	if err := m.bridge.Cleanup(); err != nil {
// 		return err
// 	}
// 	return m.overlay.Cleanup()
// }
