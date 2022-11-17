package main

import (
	"github.com/vishvananda/netlink"
	"net"
	"os"
)

func network() {

	if CONFIG.Network == nil {
		return
	}

	// loopback

	link, err := netlink.LinkByName("lo")
	if err != nil {
		log.Error("lo.LinkByName: ", err)
		return
	}
	err = netlink.LinkSetUp(link)
	if err != nil {
		log.Error("lo.LinkSetUp: ", err)
	}
	err = netlink.AddrAdd(link, &netlink.Addr{IPNet: &net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}})
	if err != nil {
		log.Warn("lo.AddrAdd: ", err)
	}



	// eth0
	link, err = netlink.LinkByName("eth0")
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
	addr, err := netlink.ParseAddr(CONFIG.Network.Ip4)
	if err == nil {
		err = netlink.AddrAdd(link, addr)
		if err != nil {
			log.Error("netlink.AddrAdd: ", err)
			return
		}
	}

	// Set up the IPv6 address
	addr, err = netlink.ParseAddr(CONFIG.Network.Ip6)
	if err == nil {
		err = netlink.AddrAdd(link, addr)
		if err != nil {
			log.Error("netlink.AddrAdd: ", err)
			return
		}
	}

	// Set up gateway4
	gateway4 := net.ParseIP(CONFIG.Network.Gateway4)
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
	gateway6 := net.ParseIP(CONFIG.Network.Gateway6)
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
	f, err := os.OpenFile("/etc/resolv.conf", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer f.Close()
	for _, nameserver := range CONFIG.Network.Nameservers {
		f.WriteString("nameserver " + nameserver + "\n")
	}
	f.WriteString("search " + CONFIG.Pod.Namespace + "\n")
}
