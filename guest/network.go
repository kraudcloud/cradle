// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/vishvananda/netlink"
)

func network() {

	if CONFIG.Network == nil {
		return
	}

	networkLoopback()
	networkInterfaces()
	networkNameservers()
}

func networkInterfaces() {

	ifaceMap := make(map[string]string)

	links, err := netlink.LinkList()
	if err != nil {
		log.Error("netlink.LinkList: ", err)
		return
	}

	for _, link := range links {
		mac := link.Attrs().HardwareAddr.String()
		name := link.Attrs().Name
		ifaceMap[mac] = name
	}

	for i, iface := range CONFIG.Network.Interfaces {

		ifname := ifaceMap[fmt.Sprintf("a0:b2:af:af:af:%02x", i)]
		if ifname == "" {
			log.Error(fmt.Sprintf("could not find interface with mac a0:b2:af:af:af:%02x", i))
			continue
		}

		eth, err := netlink.LinkByName(ifname)
		if err != nil {
			log.Error("netlink.fabric.LinkByName: ", err)
			continue
		}

		err = netlink.LinkSetName(eth, iface.Name)
		if err != nil {
			log.Error("netlink.fabric.LinkSetName: ", err)
			continue
		}

		eth0, err := netlink.LinkByName(iface.Name)
		if err != nil {
			log.Error("netlink.fabric.LinkByName (after rename): ", err)
			continue
		}

		err = netlink.LinkSetUp(eth0)
		if err != nil {
			log.Error("netlink.fabric.LinkSetUp: ", err)
		}

		//set mtu
		err = netlink.LinkSetMTU(eth0, 1400)
		if err != nil {
			log.Error("netlink.fabric.LinkSetMTU: ", err)
		}

		// Set up the addresses
		for _, ip := range iface.GuestIPs {
			addr, err := netlink.ParseAddr(ip)
			if err != nil {
				log.Errorf("net.ParseIP: %s is not a valid IP address: %s", ip, err)
				continue
			}
			err = netlink.AddrReplace(eth0, addr)
			if err != nil {
				log.Errorf("netlink.fabric.AddrAdd (%s): %s", addr.String(), err)
			}
		}

		// set routes

		for _, r := range iface.Routes {

			rr := &netlink.Route{
				LinkIndex: eth0.Attrs().Index,
			}

			gateway := net.ParseIP(r.Via)
			if gateway != nil {
				rr.Gw = gateway
			} else {
				rr.Scope = netlink.SCOPE_LINK
			}

			dst, err := netlink.ParseAddr(r.Destination)
			if err == nil {
				rr.Dst = dst.IPNet
			}

			err = netlink.RouteReplace(rr)
			if err != nil {
				log.Error("netlink.fabric.RouteAdd: ", err)
			}
		}

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

	// srv

	//FIXME this doesnt do the right thing
	// err = netlink.RouteReplace(&netlink.Route{
	// 	LinkIndex: lo.Attrs().Index,
	// 	Dst: &net.IPNet{
	// 		IP:   []byte{0xfd, 0xcc, 0xc1, 0x0d,  0, 0, 0, 0,  0, 0, 0, 0,  0, 0, 0, 0},
	// 		Mask: net.CIDRMask(32, 128),
	// 	},
	// 	Scope: netlink.SCOPE_LINK,
	// 	Table: 255, // local. why does this not work?
	// })
	// if err != nil {
	// 	log.Error("netlink.srv.RouteAdd6: ", err)
	// }

	cmd := exec.Command("/sbin/ip", "-6", "route", "add", "local", "fdcc:c10d::/32", "dev", "lo")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Error("ip.srv.RouteAdd6: ", err)
	}
}

func networkNameservers() {

	// set up nameservers
	os.MkdirAll("/etc/", 0755)
	f, err := os.OpenFile("/etc/resolv.conf", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Error("os.OpenFile (/etc/resolv.conf): ", err)
		return
	}
	defer f.Close()

	for _, nameserver := range CONFIG.Network.Nameservers {
		f.WriteString("nameserver " + nameserver + "\n")
	}
}
