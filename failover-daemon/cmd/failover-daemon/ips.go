package main

import (
	"encoding/json"
	"fmt"
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
	token        Token
	mu           sync.Mutex
	targetServer *Server
	server       *Server
}

func BadRequest(w http.ResponseWriter, why string) {
	w.WriteHeader(http.StatusBadRequest)
}

func MakeCommonRes(addr *netip.Addr, state *IPState) CommonResponse {
	res := CommonResponse{}
	res.Ip = addr.String()
	res.Netmask = addr.Zone()
	res.ServerIp = state.server.v4.String()
	res.ServerIpv6Net = state.server.v6.String()
	res.Status = "ready"
	res.ServerNumber = state.server.id

	return res
}

func SendRes(w http.ResponseWriter, res CommonResponse) {
	w.WriteHeader(200)
	enc := json.NewEncoder(w)
	enc.Encode(res)
}

func Init(config Config) (*http.ServeMux, error) {
	d := new(Daemon)
	d.ips = map[netip.Addr]*IPState{}
	d.servers = map[int]*Server{}
	d.serverIPs = map[netip.Addr]*Server{}

	routing := new(IPRoute2)

	for id, serverCfg := range config.Servers {
		server := new(Server)
		server.id = id
		// TODO: assert version
		server.v4 = serverCfg.Main.V4
		server.v6 = serverCfg.Main.V6

		d.servers[id] = server
		d.serverIPs[server.v4] = server
		d.serverIPs[server.v6] = server

		d.ips[serverCfg.Failover.V4] = new(IPState)
		d.ips[serverCfg.Failover.V4].token = serverCfg.Token
		d.ips[serverCfg.Failover.V4].server = server
		fmt.Printf("Server %d add failover %s\n", id,
			serverCfg.Failover.V4.String())

		d.ips[serverCfg.Failover.V6] = new(IPState)
		d.ips[serverCfg.Failover.V6].token = serverCfg.Token
		d.ips[serverCfg.Failover.V6].server = server
		fmt.Printf("Server %d add failover %s\n", id,
			serverCfg.Failover.V6.String())
	}

	for ip, ipState := range d.ips {
		current, err := routing.GetRoute(ip)
		if err != nil {
			return nil, err
		}

		if current == nil {
			continue
		}

		if d.serverIPs[*current] != nil {
			fmt.Sprintf("Imported: %s points to %s\n", ip, *current)
			ipState.targetServer = d.serverIPs[*current]
		}
	}

	commonHandleIP := func(w http.ResponseWriter, r *http.Request) (*netip.Addr, *IPState) {
		ipStr := r.PathValue("ip")
		ip, err := netip.ParseAddr(ipStr)
		if err != nil {
			BadRequest(w, "No valid IP")
			return nil, nil
		}

		fmt.Printf("Lookup %s\n", ip.String())

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
			res := MakeCommonRes(ip, ipState)
			if ipState.targetServer != nil {
				if ip.Is4() {
					res.ActiveServerIp = ipState.targetServer.v4.String()
				} else {
					res.ActiveServerIp = ipState.targetServer.v6.String()
				}
			}

			SendRes(w, res)
			return
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
			if newTargetServer == nil {
				BadRequest(w, "Target IP not a valid server")
				return
			}

			ipState.mu.Lock()
			defer ipState.mu.Unlock()

			ipState.targetServer = newTargetServer

			res := MakeCommonRes(ip, ipState)
			if ip.Is4() {
				res.ActiveServerIp = newTargetServer.v4.String()
			} else {
				res.ActiveServerIp = newTargetServer.v6.String()
			}

			err = routing.ReplaceRoute(*ip, newTargetIP)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Could not replace route"))
				return
			}

			SendRes(w, res)
			return
		}
	})
	mux.HandleFunc("DELETE /{ip}", func(w http.ResponseWriter, r *http.Request) {
		ip, ipState := commonHandleIP(w, r)
		if ipState != nil {
			ipState.mu.Lock()
			defer ipState.mu.Unlock()

			ipState.targetServer = nil

			res := MakeCommonRes(ip, ipState)
			res.ActiveServerIp = ""

			err := routing.RemoveRoute(*ip)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Could not remove route"))
				return
			}

			SendRes(w, res)
			return
		}
	})

	return &mux, nil
}
