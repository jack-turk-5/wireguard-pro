#!/usr/bin/env sh

/usr/bin/podman exec wireguard-pro \
iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE && \
iptables -A FORWARD -i wg0 -o eth0 -j ACCEPT && \
iptables -A FORWARD -i eth0 -o wg0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT