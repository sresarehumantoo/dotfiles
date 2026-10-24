#!/usr/bin/env bash
# Print the Homebrew formula for a dfinstall release, from its checksums.txt.
#
# Usage: brew-formula.sh <tag> <checksums.txt> [download-base-url]
#
# The formula installs the same tar.gz archives `dfinstall update` uses, so a
# release binary running from a Cellar knows it belongs to Homebrew. The base
# URL defaults to the GitHub release; Homebrew's own HOMEBREW_ARTIFACT_DOMAIN
# redirects it through a mirror.
set -euo pipefail
shopt -s inherit_errexit

usage() { sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; }
case "${1:-}" in -h | --help) usage; exit 0 ;; esac
if [[ $# -lt 2 ]]; then
    usage >&2
    exit 1
fi

tag="$1"
checksums="$2"
version="${tag#v}"
base="${3:-https://github.com/sresarehumantoo/dotfiles/releases/download/$tag}"

sha() {
    local name="dfinstall_${version}_$1_$2.tar.gz" sum
    sum="$(awk -v n="$name" '$2 == n { print $1 }' "$checksums")"
    if [[ ! "$sum" =~ ^[0-9a-f]{64}$ ]]; then
        echo "brew-formula: no sha256 for $name in $checksums" >&2
        exit 1
    fi
    printf '%s' "$sum"
}

block() {
    local sum
    sum="$(sha "$1" "$2")"
    printf '      url "%s/dfinstall_%s_%s_%s.tar.gz"\n' "$base" "$version" "$1" "$2"
    printf '      sha256 "%s"' "$sum"
}

# Computed up front: a failure inside the heredoc's substitutions would not
# stop the script.
darwin_arm64="$(block darwin arm64)"
darwin_amd64="$(block darwin amd64)"
linux_arm64="$(block linux arm64)"
linux_amd64="$(block linux amd64)"

cat <<RUBY
class Dfinstall < Formula
  desc "Dotfiles installer and manager"
  homepage "https://github.com/sresarehumantoo/dotfiles"
  version "$version"
  license "MIT"

  on_macos do
    on_arm do
$darwin_arm64
    end
    on_intel do
$darwin_amd64
    end
  end

  on_linux do
    depends_on "git"

    on_arm do
$linux_arm64
    end
    on_intel do
$linux_amd64
    end
  end

  def install
    bin.install "dfinstall", "dfinstall-mcp"
  end

  test do
    assert_match "(brew)", shell_output("#{bin}/dfinstall --version")
  end
end
RUBY
