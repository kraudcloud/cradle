// Copyright (c) 2020-present devguard GmbH

package main

import (
	"github.com/vishvananda/netlink"
	"net"
	"os"
)

func network() {

	networkLoopback()
	networkIP()

	networkK8sNameservers()
}

func networkIP() {

	eth0, err := netlink.LinkByName("eth0")
	if err != nil {
		log.Error("netlink.fabric.LinkByName(eth0): ", err)
	}

	// set link up
	err = netlink.LinkSetUp(eth0)
	if err != nil {
		log.Error("netlink.fabric.LinkSetUp: ", err)
	}

	//set mtu
	err = netlink.LinkSetMTU(eth0, 1400)
	if err != nil {
		log.Error("netlink.fabric.LinkSetMTU: ", err)
	}

	for _, addr := range CONFIG.Network.IP4 {
		addr, err := netlink.ParseAddr(addr)
		if err != nil {
			log.Errorf("netlink.ParseAddr(%s): %s", addr, err)
			continue
		}
		err = netlink.AddrReplace(eth0, addr)
		if err != nil {
			log.Errorf("netlink.AddrReplace(%s): %s", addr.String(), err)
		}
	}

	for _, addr := range CONFIG.Network.IP6 {
		addr, err := netlink.ParseAddr(addr)
		if err != nil {
			log.Errorf("netlink.ParseAddr(%s): %s", addr, err)
			continue
		}
		err = netlink.AddrReplace(eth0, addr)
		if err != nil {
			log.Errorf("netlink.AddrReplace(%s): %s", addr.String(), err)
		}
	}

	gw4 := net.ParseIP(CONFIG.Network.GW4)
	if gw4 == nil {
		log.Errorf("net.ParseIP(%s): invalid", CONFIG.Network.GW4)
	}
	err = netlink.RouteReplace(&netlink.Route{
		LinkIndex: eth0.Attrs().Index,
		Dst: &net.IPNet{
			IP:   gw4,
			Mask: net.CIDRMask(32, 32),
		},
		Scope: netlink.SCOPE_LINK,
	})
	if err != nil {
		log.Errorf("netlink.RouteReplace1(%s): %s ", gw4.String(), err)
	}
	err = netlink.RouteReplace(&netlink.Route{
		Dst:   &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)},
		Gw:    gw4,
		Scope: netlink.SCOPE_UNIVERSE,
	})
	if err != nil {
		log.Errorf("netlink.RouteReplace2(%s): %s ", gw4.String(), err)
	}

	gw6 := net.ParseIP(CONFIG.Network.GW6)
	if gw4 == nil {
		log.Errorf("net.ParseIP(%s): invalid", CONFIG.Network.GW6)
	}
	err = netlink.RouteReplace(&netlink.Route{
		LinkIndex: eth0.Attrs().Index,
		Dst: &net.IPNet{
			IP:   gw6,
			Mask: net.CIDRMask(128, 128),
		},
		Scope: netlink.SCOPE_LINK,
	})
	if err != nil {
		log.Errorf("netlink.RouteReplace1(%s): %s ", gw6.String(), err)
	}

	err = netlink.RouteReplace(&netlink.Route{
		Dst:   &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)},
		Gw:    gw6,
		Scope: netlink.SCOPE_UNIVERSE,
	})

	if err != nil {
		log.Errorf("netlink.RouteReplace2(%s): %s ", gw6.String(), err)
	}

}

func networkLoopback() {

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		log.Error("lo.LinkByName: ", err)
		return
	}
	err = netlink.LinkSetUp(lo)
	if err != nil {
		log.Error("lo.LinkSetUp: ", err)
	}
	err = netlink.AddrAdd(lo, &netlink.Addr{IPNet: &net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}})
	if err != nil {
		//log.Warn("lo.AddrAdd: ", err)
	}
}

func networkK8sNameservers() {
	os.MkdirAll("/etc/", 0755)
	f, err := os.OpenFile("/etc/resolv.conf", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Error("os.OpenFile (/etc/resolv.conf): ", err)
		return
	}
	defer f.Close()

	if CONFIG.Network.SearchDomain != "" {
		f.WriteString("search " + CONFIG.Network.SearchDomain + "\n")
	}

	for _, ns := range CONFIG.Network.Nameservers {
		f.WriteString("nameserver " + ns + "\n")
	}

	f.WriteString("options ndots:5\n")
}
