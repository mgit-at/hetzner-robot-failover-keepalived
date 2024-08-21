package main

import "net/netip"

type Routing interface {
	ReplaceRoute(failoverIP netip.Addr, targetIP netip.Addr) error
	RemoveRoute(failoverIP netip.Addr) error
	GetRoute(failoverIP netip.Addr) (netip.Addr, error)
}
