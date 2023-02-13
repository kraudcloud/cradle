#!/bin/sh
set -ex


iff=$1

ip link set $iff up
ip addr add fddd::1/64 dev $iff
ip -6 route add fddd::2/128 dev $iff

ip6tables -I INPUT -i $iff -j ACCEPT
ip6tables -I FORWARD -i $iff -j ACCEPT
ip6tables -I FORWARD -o $iff -j ACCEPT
ip6tables -t nat -C POSTROUTING -o wlp3s0 -j MASQUERADE 2>/dev/null || ip6tables -t nat -A POSTROUTING -o wlp3s0 -j MASQUERADE
ip6tables -t nat -C POSTROUTING -o host -j MASQUERADE 2>/dev/null || ip6tables -t nat -A POSTROUTING -o host -j MASQUERADE

ip addr add 10.0.2.2/24 dev $iff

iptables -I INPUT -i $iff -j ACCEPT
iptables -I FORWARD -i $iff -j ACCEPT
iptables -I FORWARD -o $iff -j ACCEPT
iptables -t nat -C POSTROUTING -o wlp3s0 -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -o wlp3s0 -j MASQUERADE
iptables -t nat -C POSTROUTING -o host -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -o host -j MASQUERADE
