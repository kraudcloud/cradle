package main

import (
	"github.com/vishvananda/netlink"
	"net"
	"os"
	"os/exec"
)

func network() {

	if CONFIG.Network == nil {
		return
	}

	// loopback

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

	// eth0: fabric

	eth0, err := netlink.LinkByName("eth0")
	if err != nil {
		log.Error("netlink.fabric.LinkByName: ", err)
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

	// Set up the IPv6 address
	addr := net.ParseIP(CONFIG.Network.FabricIp6)
	if addr == nil {
		log.Errorf("net.ParseIP(CONFIG.Network.FabricIp6): %s is not a valid IPv6 address", CONFIG.Network.FabricIp6)
	}
	err = netlink.AddrReplace(eth0, &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   addr,
			Mask: net.CIDRMask(128, 128),
		},
	})
	if err != nil {
		log.Errorf("netlink.fabric.AddrAdd6 (%s): %s", addr.String(), err)
	}

	gateway6 := net.ParseIP(CONFIG.Network.FabricGw6)
	if gateway6 == nil {
		log.Errorf("net.ParseIP(CONFIG.Network.FabricGw6): %s is not a valid IPv6 address", CONFIG.Network.FabricGw6)
	}
	err = netlink.RouteReplace(&netlink.Route{
		LinkIndex: eth0.Attrs().Index,
		Dst: &net.IPNet{
			IP:   gateway6,
			Mask: net.CIDRMask(128, 128),
		},
		Scope: netlink.SCOPE_LINK,
	})
	if err != nil {
		log.Error("netlink.fabric.RouteAdd6: ", err)
	}

	route := netlink.Route{
		LinkIndex: eth0.Attrs().Index,
		Gw:        gateway6,
	}

	err = netlink.RouteAdd(&route)
	if err != nil {
		log.Error("netlink.fabric.RouteAdd6: ", err)
	}

	// Set up the IPv4 host transit address

	addr = net.ParseIP(CONFIG.Network.TransitIp4)
	if addr == nil {
		log.Errorf("net.ParseIP(CONFIG.Network.TransitIp4): %s is not a valid IPv4 address", CONFIG.Network.TransitIp4)
	}

	err = netlink.AddrReplace(eth0, &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   addr,
			Mask: net.CIDRMask(32, 32),
		},
	})
	if err != nil {
		log.Errorf("netlink.fabric.AddrAdd4 (%s): %s", addr.String(), err)
	}

	// Set up the IPv4 host transit gateway

	gateway4 := net.ParseIP(CONFIG.Network.TransitGw4)
	if gateway4 == nil {
		log.Errorf("net.ParseIP(CONFIG.Network.TransitGw4): %s is not a valid IPv4 address", CONFIG.Network.TransitGw4)
	}
	err = netlink.RouteReplace(&netlink.Route{
		LinkIndex: eth0.Attrs().Index,
		Dst: &net.IPNet{
			IP:   gateway4,
			Mask: net.CIDRMask(32, 32),
		},
		Scope: netlink.SCOPE_LINK,
	})

	if err != nil {
		log.Error("netlink.fabric.RouteAdd4: ", err)
	}

	err = netlink.RouteAdd(&netlink.Route{
		LinkIndex: eth0.Attrs().Index,
		Gw:        gateway4,
	})
	if err != nil {
		log.Error("netlink.fabric.RouteAdd4: ", err)
	}

	// eth1: legacy vpc

	eth1, err := netlink.LinkByName("eth1")
	if err != nil {
		log.Error("netlink.LinkByName: ", err)
		return
	}

	//set mtu
	err = netlink.LinkSetMTU(eth1, 1200)

	// set link up
	err = netlink.LinkSetUp(eth1)
	if err != nil {
		log.Error("netlink.LinkSetUp: ", err)
	}

	//set mtu
	err = netlink.LinkSetMTU(eth1, 1200)
	if err != nil {
		log.Error("netlink.LinkSetMTU: ", err)
	}

	// Set up the IPv4 address
	addr2, err := netlink.ParseAddr(CONFIG.Network.Ip4)
	if err == nil {
		err = netlink.AddrAdd(eth1, addr2)
		if err != nil {
			log.Errorf("netlink.AddrAdd4 (%s): %s", addr.String(), err)
		}
	}

	_, v4vpcnet, _ := net.ParseCIDR("10.0.0.0/8")

	// Set up gateway4
	gateway4 = net.ParseIP(CONFIG.Network.Gateway4)
	if gateway4 != nil {
		route := netlink.Route{
			LinkIndex: eth1.Attrs().Index,
			Gw:        gateway4,
			Dst:       v4vpcnet,
		}

		err = netlink.RouteAdd(&route)
		if err != nil {
			log.Error("netlink.RouteAdd4: ", err)
		}
	}

	// set up nameservers
	os.MkdirAll("/etc/", 0755)
	f, err := os.OpenFile("/etc/resolv.conf", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer f.Close()

	f.WriteString("nameserver 127.127.127.127\n")

	// for _, nameserver := range CONFIG.Network.Nameservers {
	// 	f.WriteString("nameserver " + nameserver + "\n")
	// }
}
