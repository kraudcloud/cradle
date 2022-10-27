package main

import (
	"encoding/json"
	"github.com/vishvananda/netlink"
	"net"
	"os"
)

type NetConfig struct {
	Ip4         string   `json:"ip4"`
	Gateway4    string   `json:"gateway4"`
	Ip6         string   `json:"ip6"`
	Gateway6    string   `json:"gateway6"`
	Nameservers []string `json:"nameservers"`
}

func network() {
	f, err := os.Open("/config/network.json")
	if err != nil {
		log.Error(err)
		return
	}

	var netconfig NetConfig
	err = json.NewDecoder(f).Decode(&netconfig)
	if err != nil {
		log.Error("/config/network.json: ", err)
		return
	}

	// Set up the network
	link, err := netlink.LinkByName("eth0")
	if err != nil {
		log.Error("netlink.LinkByName: ", err)
		return
	}

	// set link up
	err = netlink.LinkSetUp(link)
	if err != nil {
		log.Error("netlink.LinkSetUp: ", err)
	}

	// Set up the IPv4 address
	addr, err := netlink.ParseAddr(netconfig.Ip4)
	if err == nil {
		err = netlink.AddrAdd(link, addr)
		if err != nil {
			log.Error("netlink.AddrAdd: ", err)
			return
		}
	}

	// Set up the IPv6 address
	addr, err = netlink.ParseAddr(netconfig.Ip6)
	if err == nil {
		err = netlink.AddrAdd(link, addr)
		if err != nil {
			log.Error("netlink.AddrAdd: ", err)
			return
		}
	}

	// Set up gateway4
	gateway4 := net.ParseIP(netconfig.Gateway4)
	if gateway4 != nil {
		route := netlink.Route{
			LinkIndex: link.Attrs().Index,
			Gw:        gateway4,
			Dst: &net.IPNet{
				IP:   net.IPv4zero,
				Mask: net.IPMask(net.IPv4zero),
			},
		}

		err = netlink.RouteAdd(&route)
		if err != nil {
			log.Error("netlink.RouteAdd: ", err)
		}
	}

	// Set up gateway6
	gateway6 := net.ParseIP(netconfig.Gateway6)
	if gateway6 != nil {
		route := netlink.Route{
			LinkIndex: link.Attrs().Index,
			Gw:        gateway6,
			Dst: &net.IPNet{
				IP:   net.IPv6zero,
				Mask: net.IPMask(net.IPv6zero),
			},
		}

		err = netlink.RouteAdd(&route)
		if err != nil {
			log.Error("netlink.RouteAdd: ", err)
		}
	}

	// set up nameservers
	os.MkdirAll("/etc/", 0755)
	f, err = os.OpenFile("/etc/resolv.conf", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer f.Close()
	for _, nameserver := range netconfig.Nameservers {
		f.WriteString("nameserver " + nameserver + "\n")
	}

}