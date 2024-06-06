// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	cradlespec "github.com/kraudcloud/cradle/spec"
	"github.com/vishvananda/netlink"
)

type PodNetwork struct {
	GuestMac    net.HardwareAddr
	GuestIfname string
	CID         uint32
}

func (self *VM) StartNetwork() error {

	//get ipv4 addr of pod eth0

	eth0, err := netlink.LinkByName("eth0")
	if err != nil {
		return fmt.Errorf("lo.LinkByName (eth0): %w", err)
	}

	addrs4, err := netlink.AddrList(eth0, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("netlink.AddrList: %w", err)
	}

	if len(addrs4) == 0 {
		return fmt.Errorf("no ipv4 addr found on pod eth0")
	}

	// read the pods /etc/resolv.conf
	var searchDomain = ""
	var nameservers = []string{}
	resolvConf, err := os.Open("/etc/resolv.conf")
	if err == nil {
		defer resolvConf.Close()
		scanner := bufio.NewScanner(resolvConf)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "search") {
				searchDomain = strings.TrimSpace(strings.TrimPrefix(line, "search"))
			}
			if strings.HasPrefix(line, "nameserver") {
				nameservers = append(nameservers, strings.TrimSpace(strings.TrimPrefix(line, "nameserver")))
			}
		}
	}

	self.Launch.Network = cradlespec.Network{
		IP6:          []string{"fdee:face::2/128"},
		GW6:          "fdee:face::1",
		IP4:          []string{"169.254.1.2/32"},
		GW4:          "169.254.1.1",
		Nameservers:  nameservers,
		SearchDomain: searchDomain,
	}

	// take the addr4 as cid, since its 32bit and known to be unique on the host
	cid := binary.BigEndian.Uint32(addrs4[0].IP.To4())

	self.PodNetwork = &PodNetwork{
		GuestIfname: "cradle",
		GuestMac:    net.HardwareAddr{0x00, 0x52, 0x13, 0x12, 0x00, 0x02},
		CID:         cid,
	}

	return nil
}

func system(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (self *VM) StopNetwork() {
}

func (self *VM) SetupNetworkPostLaunch() error {

	system("sysctl", "-w", "net.ipv6.conf.all.forwarding=1")
	system("sysctl", "-w", "net.ipv4.ip_forward=1")
	system("sysctl", "-w", "net.ipv6.conf.all.proxy_ndp=1")
	system("sysctl", "-w", "net.ipv4.conf.eth0.proxy_arp=1")

	var err error
	//eth0, err := netlink.LinkByName("eth0")
	//if err != nil {
	//	return fmt.Errorf("failed to find pod eth0: %s", err)
	//}

	var cradleif netlink.Link

	for i := 0; i < 60; i++ {
		cradleif, err = netlink.LinkByName(self.PodNetwork.GuestIfname)
		if err == nil {
			break
		}
		log.Warnf("failed to find tap %s, retrying in 1s", self.PodNetwork.GuestIfname)
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to find tap %s after qemu was supposed to create it: %s", self.PodNetwork.GuestIfname, err)
	}

	netlink.LinkSetUp(cradleif)

	netlink.AddrReplace(cradleif, &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   net.ParseIP(self.Launch.Network.GW4),
			Mask: net.CIDRMask(32, 32),
		},
	})

	netlink.RouteReplace(&netlink.Route{
		LinkIndex: cradleif.Attrs().Index,
		Dst:       &net.IPNet{IP: net.ParseIP("169.254.1.2"), Mask: net.CIDRMask(32, 32)},
		Scope:     netlink.SCOPE_LINK,
	})

	netlink.AddrReplace(cradleif, &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   net.ParseIP(self.Launch.Network.GW6),
			Mask: net.CIDRMask(128, 128),
		},
	})

	netlink.RouteReplace(&netlink.Route{
		LinkIndex: cradleif.Attrs().Index,
		Dst:       &net.IPNet{IP: net.ParseIP("fdee:face::2"), Mask: net.CIDRMask(128, 128)},
		Scope:     netlink.SCOPE_LINK,
	})

	system("nft", "add table ip nat")

	system("nft", "add chain ip nat prerouting { type nat hook prerouting priority -100; }")
	system("nft", "add rule  ip nat prerouting iif eth0 dnat to 169.254.1.2")

	system("nft", "add chain ip nat postrouting { type nat hook postrouting priority 100; }")
	system("nft", "add rule ip nat postrouting oifname eth0 masquerade")

	system("nft", "add table ip6 nat")

	system("nft", "add chain ip6 nat prerouting { type filter hook prerouting priority raw; policy accept; }")
	//system("nft", "add rule  ip6 nat prerouting iifname eth0 dnat to fdee:face::2")

	system("nft", "add chain ip6 nat postrouting { type nat hook postrouting priority 100; }")
	system("nft", "add rule  ip6 nat postrouting oifname eth0 masquerade")

	/*
		// v6 is less broken so we can just directly route.
		// also the customers are anti-v6 so they never even look at it anyway
		// so this is how we route services

		// transit gw addr so vm can get a router via nd
		netlink.AddrReplace(cradleif, &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   net.ParseIP(self.Launch.Network.GW6),
				Mask: net.CIDRMask(128, 128),
			},
		})

		// route pod addr to vm
		netlink.RouteReplace(&netlink.Route{
			LinkIndex: cradleif.Attrs().Index,
			Dst:       &net.IPNet{IP: net.ParseIP(self.Launch.Network.IP6[0]), Mask: net.CIDRMask(128, 128)},
			Scope:     netlink.SCOPE_LINK,
		})

		// force local ns to route v6 addr instead of delivering it local
		system("ip", "-6", "route", "del", self.Launch.Network.IP6[0], "table", "local")
		system("ip", "neigh", "add", "proxy", self.Launch.Network.IP6[0], "dev", "eth0")


	*/

	//FIXME needs policy somewhere
	//err = self.Network.SetPolicy(self.Launch.Network.Policy)

	return nil
}
