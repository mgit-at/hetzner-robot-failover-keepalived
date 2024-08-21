package main

import (
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"regexp"
	"sync"
)

type IPRoute2 struct {
	mu sync.Mutex
}

var findVia = regexp.MustCompile(`(?m)via ([0-9a-f\[.;]+)`)

func (r *IPRoute2) exec(cmdstr string) error {
	cmd := exec.Command(cmdstr)
	if errors.Is(cmd.Err, exec.ErrDot) {
		cmd.Err = nil
	}

	return cmd.Run()
}

func (r *IPRoute2) fmt(ip netip.Addr, cmdstr string) string {
	if ip.Is4() {
		return "ip " + cmdstr
	} else {
		return "ip -6 " + cmdstr
	}
}

func (r *IPRoute2) ReplaceRoute(failoverIP netip.Addr, targetIP netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.exec(r.fmt(failoverIP,
		fmt.Sprintf("route replace %s via %s", failoverIP, targetIP)))
}

func (r *IPRoute2) RemoveRoute(failoverIP netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.exec(r.fmt(failoverIP,
		fmt.Sprintf("route delete %s", failoverIP)))
}

func (r *IPRoute2) GetRoute(failoverIP netip.Addr) (*netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cmdstr := r.fmt(failoverIP, fmt.Sprintf("route get %s", failoverIP))

	out, err := exec.Command(cmdstr).Output()
	if err != nil {
		return nil, err
	}

	f := findVia.Find(out)
	ip, err := netip.ParseAddr(string(f))
	if err != nil {
		return nil, err
	}
	return &ip, nil
}
