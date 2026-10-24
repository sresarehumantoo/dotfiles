FROM mitmproxy/mitmproxy:12.2.3
USER root
RUN apt-get update \
 && apt-get install -y --no-install-recommends iptables dnsmasq \
 && rm -rf /var/lib/apt/lists/*
COPY entrypoint.sh filter.py /opt/corplab/
ENTRYPOINT ["/opt/corplab/entrypoint.sh"]
