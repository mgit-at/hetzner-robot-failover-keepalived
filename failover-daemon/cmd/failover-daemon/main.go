package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
)

type Token string

type ServerConfig struct {
	Token    Token `json:"token"`
	Main     IPSet `json:"main"`
	Failover IPSet `json:"failover"`
}

type IPSet struct {
	V4 netip.Addr `json:"v4"`
	V6 netip.Addr `json:"v6"`
}

type Config struct {
	Listen  string `json:"listen"`
	Servers map[int]ServerConfig
}

func main() {
	configPath := os.Args[1]
	file, err := os.Open(configPath)
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

	mux, err := Init(config)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Listening on %s\n", config.Listen)
	err = http.ListenAndServe(config.Listen, mux)
	if err != nil {
		panic(err)
	}
}
