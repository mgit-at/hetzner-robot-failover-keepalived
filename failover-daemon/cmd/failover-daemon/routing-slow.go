package main

import (
	"net/netip"
	"sync"
	"time"
)

type slowRouting struct {
	mu        sync.Mutex
	perIPLock map[netip.Addr]*sync.Mutex
	actual    Routing
}

func NewSlowRouting(actual Routing) Routing {
	return &slowRouting{
		actual:    actual,
		perIPLock: map[netip.Addr]*sync.Mutex{},
	}
}

func wait30s() {
	time.Sleep(30 * time.Second)
}

func (s *slowRouting) getLock(ip netip.Addr) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.perIPLock[ip] == nil {
		s.perIPLock[ip] = new(sync.Mutex)
	}

	return s.perIPLock[ip]
}

func (s *slowRouting) ReplaceRoute(failoverIP netip.Addr, targetIP netip.Addr) error {
	lock := s.getLock(failoverIP)
	lock.Lock()
	wait30s()
	defer lock.Unlock()

	return s.actual.ReplaceRoute(failoverIP, targetIP)
}

func (s *slowRouting) RemoveRoute(failoverIP netip.Addr) error {
	lock := s.getLock(failoverIP)
	lock.Lock()
	wait30s()
	defer lock.Unlock()

	return s.actual.RemoveRoute(failoverIP)
}

func (s *slowRouting) GetRoute(failoverIP netip.Addr) (*netip.Addr, error) {
	return s.actual.GetRoute(failoverIP)
}
