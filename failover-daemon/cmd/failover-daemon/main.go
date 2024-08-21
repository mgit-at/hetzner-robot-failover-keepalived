package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"os"
)

type Token string

type ServerConfig struct {
	v4 netip.Addr
	v6 netip.Addr
}

type Config struct {
	Listen  string           `json:"listen"`
	IPs     map[string]Token `json:"ips"`
	Servers map[int]ServerConfig
}

func main() {
	config := os.Args[1]
	file, err := os.Open(config)
	if err != nil {
		panic(err)
	}

	configbyte, err := io.ReadAll(file)
	if err != nil {
		panic(err)
	}

	var config Config
	err = json.Unmarshal(configbyte, &config)
	if err != nil {
		panic(err)
	}

	http.ListenAndServe()

	/*http.Server{Addr: }
	http.ListenAndServe()*/

}
