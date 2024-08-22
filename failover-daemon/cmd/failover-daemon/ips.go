package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"sync"
)

type Daemon struct {
	ips       map[netip.Addr]*IPState
	servers   map[int]*Server
	serverIPs map[netip.Addr]*Server
}

type CommonResponse struct {
	Ip             string  `json:"ip"`
	Netmask        string  `json:"netmask"`
	Status         string  `json:"status"`
	ServerIp       string  `json:"server_ip"`
	ServerIpv6Net  string  `json:"server_ipv6_net"`
	ServerNumber   int     `json:"server_number"`
	ActiveServerIp *string `json:"active_server_ip"`
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

type MessageError struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Message struct {
	Error *MessageError `json:"error,omitempty"`
}

func SendJSON(w http.ResponseWriter, msg any) {
	enc := json.NewEncoder(w)
	enc.Encode(msg)
}

func BadRequest(w http.ResponseWriter, why string) {
	fmt.Printf("[BadRequest] %s\n", why)
	w.WriteHeader(http.StatusBadRequest)

	SendJSON(w, Message{
		Error: &MessageError{
			Status:  http.StatusBadRequest,
			Code:    "BAD_REQUEST",
			Message: why,
		},
	})
}

func NotFound(w http.ResponseWriter) {
	// {"error":{"status":404,"code":"NOT_FOUND","message":"Not Found"}}
	w.WriteHeader(http.StatusNotFound)

	SendJSON(w, Message{
		Error: &MessageError{
			Status:  http.StatusNotFound,
			Code:    "NOT_FOUND",
			Message: "Not Found",
		},
	})
}

func Unauthorized(w http.ResponseWriter, why string) {
	// {"error":{"status":401,"code":"UNAUTHORIZED","message":"Unauthorized"}}
	fmt.Printf("[Unauthorized] %s\n", why)
	w.WriteHeader(http.StatusUnauthorized)

	SendJSON(w, Message{
		Error: &MessageError{
			Status:  http.StatusUnauthorized,
			Code:    "UNAUTHORIZED",
			Message: why,
		},
	})
}

func Conflict(w http.ResponseWriter, code string, why string) {
	// JSON format see Conflict(...) calls
	w.WriteHeader(http.StatusConflict)

	SendJSON(w, Message{
		Error: &MessageError{
			Status:  http.StatusConflict,
			Code:    code,
			Message: why,
		},
	})
}

func MakeCommonRes(addr *netip.Addr, state *IPState) CommonResponse {
	res := CommonResponse{}
	res.Ip = addr.String()
	if addr.Is4() {
		res.Netmask = "255.255.255.255"
	} else {
		res.Netmask = "ffff:ffff:ffff:ffff::"
	}
	res.ServerIp = state.server.v4.String()
	res.ServerIpv6Net = state.server.v6.String()
	res.Status = "ready"
	res.ServerNumber = state.server.id

	return res
}

type FailoverMessage struct {
	Res CommonResponse `json:"failover"`
}

func SendRes(w http.ResponseWriter, res CommonResponse) {
	w.WriteHeader(200)
	enc := json.NewEncoder(w)
	enc.Encode(FailoverMessage{
		Res: res,
	})
}

const authBasic = "Basic "

func Init(config Config) (*http.ServeMux, error) {
	d := new(Daemon)
	d.ips = map[netip.Addr]*IPState{}
	d.servers = map[int]*Server{}
	d.serverIPs = map[netip.Addr]*Server{}

	var routing Routing

	routing = NewIPRoute2()
	if os.Getenv("SLOW_ROUTING") != "" {
		fmt.Printf("Slow route changes enabled!\n")
		routing = NewSlowRouting(routing)
	}

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
			fmt.Printf("Imported: %s points to %s\n", ip, *current)
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

		fmt.Printf("%s /%s\n", r.Method, ip.String())

		ipState := d.ips[ip]
		if ipState == nil {
			NotFound(w)
			return nil, nil
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			Unauthorized(w, "Auth empty")
			return nil, nil
		}

		if auth[0:len(authBasic)] != authBasic {
			Unauthorized(w, "Auth not basic")
			return nil, nil
		}

		authCreds, err := base64.StdEncoding.DecodeString(auth[len(authBasic):])
		if err != nil {
			Unauthorized(w, "Auth decode failed")
			return nil, nil
		}

		compAuth := strconv.Itoa(ipState.server.id) + ":" + string(ipState.token)
		if compAuth != string(authCreds) {
			Unauthorized(w, "Auth wrong")
			return nil, nil
		}

		return &ip, ipState
	}

	// NOTE: must match Hetzner's API
	// See https://robot.hetzner.com/doc/webservice/de.html#failover
	mux := http.ServeMux{}
	mux.HandleFunc("GET /{ip}", func(w http.ResponseWriter, r *http.Request) {
		ip, ipState := commonHandleIP(w, r)
		if ipState != nil {
			res := MakeCommonRes(ip, ipState)
			if ipState.targetServer != nil {
				if ip.Is4() {
					v4 := ipState.targetServer.v4.String()
					res.ActiveServerIp = &v4
				} else {
					v6 := ipState.targetServer.v6.String()
					res.ActiveServerIp = &v6
				}
			}

			// "processing" when state.mu.TryLock() -> false
			lockSuccess := ipState.mu.TryLock()
			if lockSuccess {
				ipState.mu.Unlock()
			} else {
				res.Status = "processing"
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

			if ipState.targetServer != nil && ipState.targetServer.id == newTargetServer.id {
				// {"error":{"status":409,"code":"FAILOVER_ALREADY_ROUTED","message":"The failover ip is already routed to the selected server"}}
				Conflict(w, "FAILOVER_ALREADY_ROUTED", "The failover ip is already routed to the selected server")
				return
			}

			if !ipState.mu.TryLock() {
				// {"error":{"status":409,"code":"FAILOVER_LOCKED","message":"The failover ip can not be set up due to an active lock."}}
				Conflict(w, "FAILOVER_LOCKED", "The failover ip can not be set up due to an active lock.")
				return
			}

			defer ipState.mu.Unlock()

			ipState.targetServer = newTargetServer

			res := MakeCommonRes(ip, ipState)
			if ip.Is4() {
				v4 := newTargetServer.v4.String()
				res.ActiveServerIp = &v4
			} else {
				v6 := newTargetServer.v6.String()
				res.ActiveServerIp = &v6
			}

			err = routing.ReplaceRoute(*ip, newTargetIP)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Could not replace route"))
				fmt.Printf("%s -> %s: Could not replace route\n", ip, newTargetIP)
				return
			}

			SendRes(w, res)
			return
		}
	})
	mux.HandleFunc("DELETE /{ip}", func(w http.ResponseWriter, r *http.Request) {
		ip, ipState := commonHandleIP(w, r)
		if ipState != nil {
			if !ipState.mu.TryLock() {
				Conflict(w, "FAILOVER_LOCKED", "The failover ip can not be set up due to an active lock.")
				return
			}
			defer ipState.mu.Unlock()

			// Double-delete is a no-op on Hetzner, so it's a no-op here aswell

			ipState.targetServer = nil

			res := MakeCommonRes(ip, ipState)

			err := routing.RemoveRoute(*ip)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Could not remove route"))
				fmt.Printf("%s: Could not delete route\n", ip)
				return
			}

			SendRes(w, res)
			return
		}
	})

	return &mux, nil
}
