#!/usr/bin/env bash
# Add .deb packages to a signed apt repository and regenerate its indexes.
#
# Usage: APT_SIGNING_KEY_ID=<gpg key id> apt-repo.sh <repo-dir> <package.deb>...
#
# The repository has one suite, "stable", with component "main", and keeps the
# newest three versions. The signing key must already be in the gpg keyring;
# its public half is written to <repo-dir>/dfinstall-archive-keyring.gpg.
# Clients add it with:
#
#   deb [signed-by=/etc/apt/keyrings/dfinstall.gpg] <repo-url> stable main
set -euo pipefail
shopt -s inherit_errexit

usage() { sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; }
case "${1:-}" in -h | --help) usage; exit 0 ;; esac
if [[ $# -lt 2 || -z "${APT_SIGNING_KEY_ID:-}" ]]; then
    usage >&2
    exit 1
fi
for cmd in apt-ftparchive gpg; do
    command -v "$cmd" >/dev/null || { echo "apt-repo: $cmd is required" >&2; exit 1; }
done

repo="$1"
shift
keep=3
archs=(amd64 arm64)
pool="pool/main/d/dfinstall"

mkdir -p "$repo/$pool"
for deb in "$@"; do
    [[ "$(basename "$deb")" =~ ^dfinstall_[0-9][^_]*_[a-z0-9]+\.deb$ ]] || { echo "apt-repo: unexpected package name $deb" >&2; exit 1; }
    cp -- "$deb" "$repo/$pool/"
done

cd -- "$repo"

# dfinstall_<version>_<arch>.deb; drop every version past the newest $keep.
mapfile -t old < <(
    find "$pool" -name 'dfinstall_*.deb' -printf '%f\n' | cut -d_ -f2 | sort -uVr | tail -n "+$((keep + 1))"
)
for v in "${old[@]}"; do
    rm -f -- "$pool/dfinstall_${v}_"*.deb
done

dist="dists/stable"
for arch in "${archs[@]}"; do
    dir="$dist/main/binary-$arch"
    mkdir -p "$dir"
    apt-ftparchive --arch "$arch" packages "$pool" >"$dir/Packages"
    gzip -9nkf "$dir/Packages"
done

release="$(mktemp)"
trap 'rm -f "$release"' EXIT
apt-ftparchive \
    -o APT::FTPArchive::Release::Origin=dfinstall \
    -o APT::FTPArchive::Release::Label=dfinstall \
    -o APT::FTPArchive::Release::Suite=stable \
    -o APT::FTPArchive::Release::Codename=stable \
    -o "APT::FTPArchive::Release::Architectures=${archs[*]}" \
    -o APT::FTPArchive::Release::Components=main \
    release "$dist" >"$release"
cp -- "$release" "$dist/Release"

gpg --batch --yes --local-user "$APT_SIGNING_KEY_ID" --clearsign -o "$dist/InRelease" "$dist/Release"
gpg --batch --yes --local-user "$APT_SIGNING_KEY_ID" --armor --detach-sign -o "$dist/Release.gpg" "$dist/Release"
gpg --batch --yes --export "$APT_SIGNING_KEY_ID" >dfinstall-archive-keyring.gpg
