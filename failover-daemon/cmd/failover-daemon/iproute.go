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

var findVia = regexp.MustCompile(`via ([0-9a-f\[.:]+)`)

func (r *IPRoute2) exec(cmdstr ...string) error {
	fmt.Sprintf("[iproute] exec ip %s\n", cmdstr)
	cmd := exec.Command("ip", cmdstr...)
	if errors.Is(cmd.Err, exec.ErrDot) {
		cmd.Err = nil
	}

	return cmd.Run()
}

func (r *IPRoute2) fmt(ip netip.Addr, cmdstr ...string) []string {
	if ip.Is4() {
		return cmdstr
	} else {
		return append([]string{"-6"}, cmdstr...)
	}
}

func (r *IPRoute2) ReplaceRoute(failoverIP netip.Addr, targetIP netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.exec(r.fmt(failoverIP,
		"route", "replace", failoverIP.String(), "via", targetIP.String())...)
}

func (r *IPRoute2) RemoveRoute(failoverIP netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.exec(r.fmt(failoverIP,
		"route", "delete", failoverIP.String())...)
}

func (r *IPRoute2) GetRoute(failoverIP netip.Addr) (*netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cmdstr := r.fmt(failoverIP, "route", "get", failoverIP.String())

	fmt.Sprintf("[iproute] exec ip %s\n", cmdstr)
	out, err := exec.Command("ip", cmdstr...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Address not routable
			if exitErr.ExitCode() == 2 {
				return nil, nil
			}
		}
		return nil, err
	}

	// No route found
	if !findVia.Match(out) {
		return nil, nil
	}

	f := findVia.FindSubmatch(out)
	ip, err := netip.ParseAddr(string(f[1]))
	if err != nil {
		return nil, err
	}
	return &ip, nil
}
