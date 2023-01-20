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

	//set mtu
	err = netlink.LinkSetMTU(link, 1370)

	// set link up
	err = netlink.LinkSetUp(link)
	if err != nil {
		log.Error("netlink.LinkSetUp: ", err)
	}

	//set mtu
	err = netlink.LinkSetMTU(link, 1200)
	if err != nil {
		log.Error("netlink.LinkSetMTU: ", err)
	}

	// Set up the IPv4 address
	addr, err := netlink.ParseAddr(CONFIG.Network.Ip4)
	if err == nil {
		err = netlink.AddrAdd(link, addr)
		if err != nil {
			log.Errorf("netlink.AddrAdd4 (%s): %s", addr.String(), err)
		}
	}

	// Set up the IPv6 address
	addr, err = netlink.ParseAddr(CONFIG.Network.Ip6)
	if err == nil {
		err = netlink.AddrAdd(link, addr)
		if err != nil {
			log.Errorf("netlink.AddrAdd6 (%s): %s", addr.String(), err)
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
			log.Error("netlink.RouteAdd4: ", err)
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
			log.Error("netlink.RouteAdd6: ", err)
		}
	}


	// setup fabric if it exists
	link, err = netlink.LinkByName("eth1")
	if err == nil {

		// set link up
		err = netlink.LinkSetUp(link)
		if err != nil {
			log.Error("netlink.fabric.LinkSetUp: ", err)
		}

		// Set up the IPv6 address

		addr := net.ParseIP(CONFIG.Network.FabricIp6)
		if addr != nil {
			err = netlink.AddrReplace(link, &netlink.Addr{
				IPNet: &net.IPNet{
					IP:   addr,
					Mask: net.CIDRMask(128, 128),
				},
			})
			if err != nil {
				log.Errorf("netlink.fabric.AddrAdd6 (%s): %s", addr.String(), err)
			}
		}

		gateway6 := net.ParseIP(CONFIG.Network.FabricGw6)
		if gateway6 != nil {

			err = netlink.RouteReplace(&netlink.Route{
				LinkIndex: link.Attrs().Index,
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
				LinkIndex: link.Attrs().Index,
				Gw:        gateway6,
			}

			err = netlink.RouteAdd(&route)
			if err != nil {
				log.Error("netlink.fabric.RouteAdd6: ", err)
			}
		}
	}




	// set up nameservers
	os.MkdirAll("/etc/", 0755)
	f, err := os.OpenFile("/etc/resolv.conf", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer f.Close()
	for _, nameserver := range CONFIG.Network.Nameservers {
		f.WriteString("nameserver " + nameserver + "\n")
	}
}
