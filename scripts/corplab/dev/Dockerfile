# The corporate workstation. Packages come from the image build, which has
# normal network access; at run time everything goes through the proxy.
FROM debian:trixie

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates openssl curl git zsh unzip \
      nodejs npm python3 python3-pip iproute2 sudo jq procps \
 && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash dev \
 && echo 'dev ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/dev

COPY corp-net /usr/local/sbin/corp-net
ENTRYPOINT ["/usr/local/sbin/corp-net"]
