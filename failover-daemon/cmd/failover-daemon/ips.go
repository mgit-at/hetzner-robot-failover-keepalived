package main

import (
	"net/http"
	"net/netip"
	"sync"
)

type Daemon struct {
	ips       map[netip.Addr]*IPState
	servers   map[int]*Server
	serverIPs map[netip.Addr]*Server
}

type CommonResponse struct {
	Ip             string `json:"ip"`
	Netmask        string `json:"netmask"`
	Status         string `json:"status"`
	ServerIp       string `json:"server_ip"`
	ServerIpv6Net  string `json:"server_ipv6_net"`
	ServerNumber   int    `json:"server_number"`
	ActiveServerIp string `json:"active_server_ip"`
}

type Server struct {
	id int
	v4 netip.Addr
	v6 netip.Addr
}

type IPState struct {
	ident        string
	token        Token
	lock         sync.Locker
	targetServer *Server
}

func BadRequest(w http.ResponseWriter, why string) {
	w.WriteHeader(http.StatusBadRequest)
}

func Init(config Config) (*http.ServeMux, error) {
	d := new(Daemon)

	for ipStr, token := range config.IPs {
		ip, err := netip.ParseAddr(ipStr)
		if err != nil {
			return nil, err
		}
		state := new(IPState)
		state.ident = ip.StringExpanded()
		state.token = token
		state.targetServer = nil
		d.ips[ip] = state
	}

	for id, serverCfg := range config.Servers {
		server := new(Server)
		server.id = id
		// TODO: assert version
		server.v4 = serverCfg.v4
		server.v6 = serverCfg.v6

		d.servers[id] = server
		d.serverIPs[server.v4] = server
		d.serverIPs[server.v6] = server
	}
	commonHandleIP := func(w http.ResponseWriter, r *http.Request) (*netip.Addr, *IPState) {
		ipStr := r.PathValue("ip")
		ip, err := netip.ParseAddr(ipStr)
		if err != nil {
			BadRequest(w, "No valid IP")
			return nil, nil
		}

		ipState := d.ips[ip]
		if ipState == nil {
			http.NotFound(w, r)
			return nil, nil
		}

		return &ip, ipState
	}

	// NOTE: must match hetzner's API
	// See https://robot.hetzner.com/doc/webservice/de.html#failover
	mux := http.ServeMux{}
	mux.HandleFunc("GET /{ip}", func(w http.ResponseWriter, r *http.Request) {
		ip, ipState := commonHandleIP(w, r)
		if ipState != nil {
			res := new(CommonResponse)
			res.Ip = ip.String()
			if ipState.targetServer != nil {
				if ip.Is4() {
					res.ActiveServerIp = ipState.targetServer.v4.String()
				} else {
					res.ActiveServerIp = ipState.targetServer.v6.String()
				}
				res.ServerNumber = ipState.targetServer.id
			}
		}
	})
	mux.HandleFunc("POST /{ip}", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			BadRequest(w, "No valid form data")
			return
		}
		ip, ipState := commonHandleIP(w, r)
		if ipState != nil {
			newTarget := r.Form.Get("active_server_ip")
			if newTarget == "" {
				BadRequest(w, "No target IP provided")
				return
			}

			newTargetIP, err := netip.ParseAddr(newTarget)
			if err != nil {
				BadRequest(w, "Target IP not valid IP")
				return
			}

			newTargetServer := d.serverIPs[newTargetIP]
			ipState.lock.Lock()
			defer ipState.lock.Unlock()

			ipState.targetServer = newTargetServer
		}
	})
	mux.HandleFunc("DELETE /{ip}", func(w http.ResponseWriter, r *http.Request) {
		ip, ipState := commonHandleIP(w, r)
		if ipState != nil {
			ipState.lock.Lock()
			defer ipState.lock.Unlock()

			ipState.targetServer = nil
		}
	})

	return &mux, nil
}
