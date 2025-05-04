#!/usr/bin/env sh

/usr/bin/podman exec wireguard-pro \
/usr/sbin/iptables -t nat -D POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE &&
/usr/sbin/iptables -D FORWARD -i wg0 -o eth0 -j ACCEPT &&
/usr/sbin/iptables -D FORWARD -i eth0 -o wg0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT