package store

import (
	"encoding/json"
	"errors"
	"net"
	"os"
)

type Config struct {
	PanelListen   string `json:"panel_listen"`
	PublicHost    string `json:"public_host"`
	ICMPListen    string `json:"icmp_listen"`
	ICMPListen6   string `json:"icmp_listen6"`
	EndpointFamily string `json:"endpoint_family"`
	StatePath     string `json:"state_path"`
	TLSCert       string `json:"tls_cert"`
	TLSKey        string `json:"tls_key"`
	MaxSessions   int    `json:"max_sessions"`
	MaxPerUser    int    `json:"max_sessions_per_user"`
	MaxUDPPerUser int    `json:"max_udp_sessions_per_user"`
	Window        int    `json:"max_window"`
	AllowPrivate  bool   `json:"allow_private_targets"`
	ICMPPacketsPerSecond int `json:"icmp_packets_per_second"`
	ICMPBurstPackets int `json:"icmp_burst_packets"`
}

func DefaultConfig() Config {
	return Config{PanelListen: "0.0.0.0:8443", ICMPListen: "0.0.0.0", ICMPListen6:"::",EndpointFamily:"ipv4",StatePath: "/var/lib/namedicmp/state.json", MaxSessions: 256, MaxPerUser: 32, MaxUDPPerUser: 128, Window: 20000, ICMPPacketsPerSecond: 2500, ICMPBurstPackets: 32}
}
func LoadConfig(path string) (Config, error) {
	c := DefaultConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return c, err
	}
	_, _, err = net.SplitHostPort(c.PanelListen)
	if err != nil {
		return c, err
	}
	if c.MaxSessions < 1 || c.MaxSessions > 10000 || c.MaxPerUser < 1 || c.MaxUDPPerUser < 1 || c.MaxUDPPerUser > 10000 || c.Window < 8 || c.Window > 100000 || c.StatePath == "" || (c.TLSCert == "") != (c.TLSKey == "") {
		return c, errors.New("invalid config")
	}
	if c.ICMPPacketsPerSecond < 0 || c.ICMPPacketsPerSecond > 100000 || c.ICMPBurstPackets < 1 || c.ICMPBurstPackets > 1024 {return c,errors.New("invalid ICMP pacing settings")}
	if c.EndpointFamily!="ipv4"&&c.EndpointFamily!="ipv6" {return c,errors.New("invalid endpoint family")}
	return c, nil
}
func SaveConfig(path string, c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}

