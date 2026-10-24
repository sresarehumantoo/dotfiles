"""URL filtering, as a corporate firewall's policy would apply it.

/state/policy/blocklist holds one rule per line, re-read on every request:
    github.com        policy block page (403) for the host and its subdomains
    drop:github.com   the connection is reset instead
    *                 everything
    !nexus            exempt, checked before any block
"""
from pathlib import Path

from mitmproxy import http

BLOCKLIST = Path("/state/policy/blocklist")
PAGE = b"<html><body><h1>Web Page Blocked</h1><p>Access to this site is blocked by corporate policy.</p></body></html>"


def _matches(host: str, pattern: str) -> bool:
    return pattern == "*" or host == pattern or host.endswith("." + pattern)


def request(flow: http.HTTPFlow) -> None:
    try:
        rules = BLOCKLIST.read_text().split()
    except FileNotFoundError:
        return
    host = flow.request.pretty_host
    if any(_matches(host, r[1:]) for r in rules if r.startswith("!")):
        return
    for rule in rules:
        drop = rule.startswith("drop:")
        if _matches(host, rule.removeprefix("drop:")):
            if drop:
                flow.kill()
            else:
                flow.response = http.Response.make(403, PAGE, {"Content-Type": "text/html"})
            return
