# Serves a mirror of the dotfiles repo over git's dumb HTTP protocol, which
# any static web server can do. It is cloned when the image is built, which
# has normal network access; at run time it is on the corp network only.
FROM debian:trixie
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates python3 \
 && rm -rf /var/lib/apt/lists/*
RUN git clone -q --bare https://github.com/sresarehumantoo/dotfiles.git /srv/github/sresarehumantoo/dotfiles.git \
 && git -C /srv/github/sresarehumantoo/dotfiles.git update-server-info
WORKDIR /srv
CMD ["python3", "-m", "http.server", "8000"]
