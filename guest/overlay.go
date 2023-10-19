// Copyright (c) 2020-present devguard GmbH

package main

import (
	"encoding/base64"
	"fmt"
	"github.com/google/uuid"
	"github.com/vishvananda/netlink"
	"net"
	"os/exec"
	"strings"
)

func networkOverlay() {
	//ip link add overlay type ip6tnl external
	o, err := exec.Command("/sbin/ip", "link", "add", "overlay", "type", "ip6tnl", "external").CombinedOutput()

	// err := netlink.LinkAdd(&netlink.Ip6tnl{
	// 	LinkAttrs: netlink.LinkAttrs{
	// 		Name: "overlay",
	// 	},
	//  TODO dunno how to set externally controlled flag
	// })
	if err != nil {
		log.Error(fmt.Errorf("failed to create overlay: %w : %s", err, string(o)))
		return
	}

	eth, err := netlink.LinkByName("overlay")
	if err != nil {
		log.Error(fmt.Errorf("netlink.LinkByName (overlay): %w", err))
		return
	}

	netlink.LinkSetUp(eth)

}

func networkMakeOverlay(net4 string, net6 string) (netlink.Link, error) {

	eth, err := netlink.LinkByName("overlay")
	if err != nil {
		return nil, fmt.Errorf("netlink.LinkByName (overlay): %w", err)
	}

	if net4 != "" {

		ip, _, err := net.ParseCIDR(net4)
		if err != nil {
			ip = net.ParseIP(net4)
			if ip == nil {
				log.Errorf("cannot parse overlay ip: (%s): %s", net4, err)
			}
		}
		if ip != nil {
			err = netlink.AddrReplace(eth, &netlink.Addr{
				IPNet: &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(32, 32),
				},
			})
			if err != nil {
				log.Errorf("netlink.AddrAdd (%s): %s", ip.String(), err)
			}
		}
	}

	if net6 != "" {

		ip, _, err := net.ParseCIDR(net6)
		if err != nil {
			ip = net.ParseIP(net6)
			if ip == nil {
				log.Errorf("cannot parse overlay ip: (%s): %s", net6, err)
			}
		}
		if ip != nil {
			err = netlink.AddrReplace(eth, &netlink.Addr{
				IPNet: &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(128, 128),
				},
			})
			if err != nil {
				log.Errorf("netlink.AddrAdd (%s): %s", ip.String(), err)
			}

		}
	}

	return eth, nil
}

func withMask(in string) string {
	if strings.Contains(in, "/") {
		return in
	}
	if strings.Contains(in, ":") {
		return in + "/128"
	}
	return in + "/32"
}

func UpdateOverlay(vv *Vpc) {

	log.Infof("DEBUG UpdateOverlay %d", len(vv.Pods))

	myuuid, err := uuid.Parse(CONFIG.ID)
	if err != nil {
		log.Error("cannot parse CONFIG.ID: ", CONFIG.ID, err)
		return
	}
	myshortid := base64.RawURLEncoding.EncodeToString(myuuid[:])

	hasRoutes := make(map[string]netlink.Route)

	for shid, pod := range vv.Pods {
		if shid != myshortid {
			continue
		}

		for _, overlay := range pod.Overlays {

			eth, err := networkMakeOverlay(overlay.IP4, overlay.IP6)
			if err != nil {
				if err != nil {
					log.Error(err)
					continue
				}
			}

			routes, err := netlink.RouteList(eth, netlink.FAMILY_V6)
			if err != nil {
				log.Error("netlink.RouteList (overlay): ", err)
			} else {
				for _, route := range routes {
					if route.Protocol != 10 {
						continue
					}
					hasRoutes[route.Dst.String()] = route
				}
			}

			routes, err = netlink.RouteList(eth, netlink.FAMILY_V4)
			if err != nil {
				log.Error("netlink.RouteList (overlay): ", err)
			} else {
				for _, route := range routes {
					if route.Protocol != 10 {
						continue
					}
					hasRoutes[route.Dst.String()] = route
				}
			}
		}
	}

	keepRoutes := make(map[string]bool)

	for shid, pod := range vv.Pods {
		if shid == myshortid {
			continue
		}

		for _, overlay := range pod.Overlays {

			if overlay.IP4 != "" {
				dst := withMask(overlay.IP4)
				keepRoutes[dst] = true

				if hasRoutes[overlay.IP4].Gw == nil ||
					hasRoutes[overlay.IP4].Gw.String() != pod.IP6 {

					o, err := exec.Command("/sbin/ip", "route", "replace",
						dst, "dev", "overlay", "encap", "ip6", "dst", pod.IP6, "proto", "10").CombinedOutput()

					if err != nil {
						log.Error("netlink.RouteReplace (overlay) : ", err, string(o))
					}
				}
			}

			if overlay.IP6 != "" {
				dst := withMask(overlay.IP4)
				keepRoutes[dst] = true

				if hasRoutes[overlay.IP6].Gw == nil ||
					hasRoutes[overlay.IP6].Gw.String() != pod.IP6 {

					o, err := exec.Command("/sbin/ip", "-6", "route", "replace",
						dst, "dev", "overlay", "encap", "ip6", "dst", pod.IP6, "proto", "10").CombinedOutput()

					if err != nil {
						log.Error("netlink.RouteReplace (overlay) : ", err, string(o))
					}
				}
			}
		}
	}

	for dst, r := range hasRoutes {
		if !keepRoutes[dst] {

			log.Infof("DEBUG no keep route: %s", dst)

			err = netlink.RouteDel(&r)
			if err != nil {
				log.Error("netlink.RouteDel (overlay): ", err)
			}
		}
	}

}
