#!/bin/sh
# Inline TLS inspection: take every web connection routed through this box,
# whatever its destination, and re-sign it with the lab CA.
set -eu

iptables -t nat -A PREROUTING -p tcp -m multiport --dports 80,443 -j REDIRECT --to-ports 8080

# DNS for the corp side. Docker's resolver in this namespace reaches outside;
# the internal network's does not.
dnsmasq --no-resolv --no-hosts --server=127.0.0.11 --except-interface=lo --bind-dynamic

mkdir -p /conf
cat /state/ca/corp-ca.key /state/ca/corp-ca.crt > /conf/mitmproxy-ca.pem
touch /tmp/ready
exec mitmdump --mode transparent --showhost --set confdir=/conf \
    --set block_global=false --set flow_detail=0 -s /opt/corplab/filter.py
