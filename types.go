package main

import "net"

type Message struct {
	Records []ServiceRecord `json:"records"`
}

type ServiceRecord struct {
	ServiceIdentity
	HostName   string   `json:"host"`
	Port       int      `json:"port"`
	IPs        []net.IP `json:"ips"`
	TXTRecords []string `json:"txt"`
}

func (r ServiceRecord) Equal(s ServiceRecord) bool {
	b := r.ServiceIdentity == s.ServiceIdentity && r.HostName == s.HostName && r.Port == s.Port
	if !b {
		return false
	}
	if len(r.IPs) != len(s.IPs) || len(r.TXTRecords) != len(s.TXTRecords) {
		return false
	}
	for i := range r.IPs {
		if !r.IPs[i].Equal(s.IPs[i]) {
			return false
		}
	}
	for i := range r.TXTRecords {
		if r.TXTRecords[i] != s.TXTRecords[i] {
			return false
		}
	}
	return true
}

type ServiceIdentity struct {
	Instance string `json:"instance"`
	Service  string `json:"service"`
	Domain   string `json:"domain"`
}
