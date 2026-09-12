package modules

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// Third-party apt signing keys rotate on the upstream's own schedule, which
// has nothing to do with the repo definition on disk. When it happens apt
// refuses the new index ("Missing key ...") and keeps serving the stale one,
// exiting 100 — while every other signal still looks healthy: the repo file is
// unchanged, the key file is present, the installed binary still runs. The
// helpers here let a converge notice the rotation and repair the keyring in
// place instead of the box quietly rotting until someone reads apt's output.

// keyProbeTimeout bounds the InRelease fetch and the gpg calls behind the
// freshness check. This runs on every converge, so it must not be able to sit
// on a blackholed mirror: an InRelease is a small signed manifest, not a
// package, and 30s is already generous.
const keyProbeTimeout = 30 * time.Second

// armorHeader marks an ASCII-armored key file. The /gpg endpoints used by
// callers of addAptRepo all serve armored keys, but a keyring installed by
// some other tool may be binary, so both have to be handled.
const armorHeader = "-----BEGIN PGP PUBLIC KEY BLOCK-----"

// ensureAptRepoKey checks the keyring at keyPath against the repo's live
// InRelease and refreshes it if a key rotation has made it useless. It
// reports whether it changed the keyring on disk.
//
// The error return is advisory: the caller downgrades it to a warning rather
// than aborting. A repo whose key genuinely cannot be repaired will fail the
// subsequent apt update loudly on its own, and turning a transient upstream
// hiccup into a failed bootstrap is the worse trade.
func ensureAptRepoKey(ctx context.Context, name, keyURL, keyPath, repoContent string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, keyProbeTimeout)
	defer cancel()

	inReleaseURL, err := aptInReleaseURL(repoContent)
	if err != nil {
		// Not fatal: an unparseable repo line means only that this check
		// cannot run, not that anything is wrong with the key.
		core.Debug("skipping %s key check: %v", name, err)
		return false, nil
	}

	inRelease, cleanup, err := fetchTemp(ctx, inReleaseURL)
	if err != nil {
		// Offline, proxied, or a mirror having a bad minute. Staying quiet
		// and doing nothing is right: we have no evidence either way.
		core.Debug("skipping %s key check, could not fetch %s: %v", name, inReleaseURL, err)
		return false, nil
	}
	defer cleanup()

	if gpgvVerify(ctx, keyPath, inRelease) {
		return false, nil
	}

	core.Notice("%s signing key no longer verifies the repo index — refreshing", name)

	newKey, cleanupKey, err := fetchTemp(ctx, keyURL)
	if err != nil {
		return false, fmt.Errorf("downloading %s signing key: %w", name, err)
	}
	defer cleanupKey()

	// Never install a key on the upstream's say-so alone. If the freshly
	// fetched key does not verify the index either, this is not a rotation,
	// and overwriting the keyring would destroy evidence while fixing
	// nothing.
	if !gpgvVerify(ctx, newKey, inRelease) {
		return false, fmt.Errorf("%s: key from %s does not verify %s either, leaving keyring alone",
			name, keyURL, inReleaseURL)
	}

	merged, err := mergeKeyrings(ctx, keyPath, newKey)
	if err != nil {
		return false, fmt.Errorf("merging %s keyring: %w", name, err)
	}
	if err := writeFileAsRoot(ctx, keyPath, merged, 0o644); err != nil {
		return false, fmt.Errorf("installing refreshed %s keyring: %w", name, err)
	}
	return true, nil
}

// mergeKeyrings returns the contents to write to keyPath so that it carries
// the new certificate without losing what was already there.
//
// Appending rather than replacing matters during a rotation window. An
// upstream may still be signing some suites with the outgoing key, and that
// key is not expired merely because a successor exists. A superseded
// certificate sitting in the keyring costs nothing; dropping one that is
// still in use breaks the repo a second time. apt reads a concatenation of
// armored certificates fine.
//
// Binary keyrings are replaced outright: armored and binary packets cannot be
// concatenated into a file that parses.
func mergeKeyrings(ctx context.Context, keyPath, newKeyPath string) ([]byte, error) {
	newKey, err := os.ReadFile(newKeyPath)
	if err != nil {
		return nil, err
	}

	existing, err := os.ReadFile(keyPath)
	if err != nil {
		return newKey, nil // no keyring yet, or unreadable: start fresh
	}
	if !isArmoredKey(existing) || !isArmoredKey(newKey) {
		return newKey, nil
	}

	// If the upstream key is already present, appending it again would just
	// grow the file on every converge.
	have, err := keyFingerprints(ctx, keyPath)
	if err != nil {
		return newKey, nil
	}
	incoming, err := keyFingerprints(ctx, newKeyPath)
	if err != nil {
		return newKey, nil
	}
	if isSubset(incoming, have) {
		return existing, nil
	}

	merged := append(bytes.TrimRight(existing, "\n"), '\n')
	return append(merged, newKey...), nil
}

// gpgvVerify reports whether signedFile carries a good signature from some
// certificate in the keyring at keyPath.
//
// gpgv cannot be pointed at an armored keyring directly: given a .asc holding
// more than one certificate it exits 2 even when the signature is good. Every
// keyring addAptRepo installs is armored, and mergeKeyrings deliberately makes
// them multi-certificate, so the keyring is always dearmored to a scratch file
// first. Getting this wrong reports every repo as broken.
func gpgvVerify(ctx context.Context, keyPath, signedFile string) bool {
	if _, err := os.Stat(keyPath); err != nil {
		return false
	}
	ring, cleanup, err := dearmorTemp(ctx, keyPath)
	if err != nil {
		return false
	}
	defer cleanup()

	// Run gpgv directly rather than through runCmd: here a nonzero exit is
	// the answer being asked for, not a failure worth printing to the user.
	cmd := exec.CommandContext(ctx, "gpgv", "--keyring", ring, signedFile)
	return cmd.Run() == nil
}

// dearmorTemp writes a binary keyring copy of keyPath to a scratch file and
// returns its path plus a cleanup func.
func dearmorTemp(ctx context.Context, keyPath string) (string, func(), error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", nil, err
	}

	if isArmoredKey(data) {
		cmd := exec.CommandContext(ctx, "gpg", "--dearmor")
		cmd.Stdin = bytes.NewReader(data)
		out, err := cmd.Output()
		if err != nil {
			return "", nil, fmt.Errorf("dearmor %s: %w", keyPath, err)
		}
		data = out
	}

	return writeTemp("dfinstall-keyring-*.gpg", data)
}

// keyFingerprints returns the primary-key fingerprints in a key file.
func keyFingerprints(ctx context.Context, keyPath string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "gpg", "--show-keys", "--with-colons", keyPath).Output()
	if err != nil {
		return nil, fmt.Errorf("reading keys from %s: %w", keyPath, err)
	}
	return parsePrimaryFingerprints(string(out)), nil
}

// parsePrimaryFingerprints pulls primary-key fingerprints out of gpg's
// colon-delimited output. A "fpr" record describes whichever "pub" or "sub"
// record preceded it, and only the primaries are wanted here.
func parsePrimaryFingerprints(colons string) []string {
	var fprs []string
	primary := false
	for _, line := range strings.Split(colons, "\n") {
		fields := strings.Split(line, ":")
		switch fields[0] {
		case "pub":
			primary = true
		case "sub":
			primary = false
		case "fpr":
			if primary && len(fields) > 9 && fields[9] != "" {
				fprs = append(fprs, fields[9])
				primary = false
			}
		}
	}
	return fprs
}

// aptKeyringUsable reports whether a keyring exists, parses, and holds at
// least one certificate that is neither expired nor revoked.
//
// Offline by design so it costs nothing in a status check. Note what it does
// NOT catch: a key rotation leaves a perfectly valid, unexpired certificate on
// disk that simply is not the one signing the repo any more. Only the
// converge-time probe in ensureAptRepoKey sees that.
func aptKeyringUsable(keyPath string) bool {
	if _, err := os.Stat(keyPath); err != nil {
		return false
	}
	out, err := exec.Command("gpg", "--show-keys", "--with-colons", keyPath).Output()
	if err != nil {
		return false
	}
	return hasLiveCertificate(string(out), time.Now())
}

// hasLiveCertificate reports whether gpg colon output describes at least one
// primary key that is usable at the given time.
func hasLiveCertificate(colons string, now time.Time) bool {
	for _, line := range strings.Split(colons, "\n") {
		fields := strings.Split(line, ":")
		if fields[0] != "pub" || len(fields) < 7 {
			continue
		}
		switch fields[1] {
		case "e", "r", "i", "d": // expired, revoked, invalid, disabled
			continue
		}
		if exp := fields[6]; exp != "" {
			secs, err := strconv.ParseInt(exp, 10, 64)
			if err != nil || !time.Unix(secs, 0).After(now) {
				continue
			}
		}
		return true
	}
	return false
}

// aptInReleaseURL derives the InRelease URL apt will fetch for a repo
// definition.
//
// Deriving it from the repo content rather than passing it in alongside keeps
// a single source of truth: the probe cannot drift away from the suite the
// repo line actually points at. Handles both formats used by callers of
// addAptRepo — one-line "deb [options] URI suite components" and deb822
// "URIs:"/"Suites:" stanzas.
func aptInReleaseURL(repoContent string) (string, error) {
	uri, suite, err := aptRepoURISuite(repoContent)
	if err != nil {
		return "", err
	}
	uri = strings.TrimRight(uri, "/")

	// A suite ending in "/" is apt's flat-repo form: the suite is a path
	// relative to the URI, with no dists/ indirection. The common spelling is
	// "./", meaning the repo root itself.
	if strings.HasSuffix(suite, "/") {
		path := strings.Trim(strings.TrimPrefix(strings.TrimSuffix(suite, "/"), "."), "/")
		if path == "" {
			return uri + "/InRelease", nil
		}
		return uri + "/" + path + "/InRelease", nil
	}
	return uri + "/dists/" + suite + "/InRelease", nil
}

func aptRepoURISuite(repoContent string) (string, string, error) {
	trimmed := strings.TrimSpace(repoContent)
	if trimmed == "" {
		return "", "", fmt.Errorf("empty repo definition")
	}
	if strings.Contains(trimmed, "URIs:") {
		return deb822URISuite(trimmed)
	}
	return oneLineURISuite(trimmed)
}

func deb822URISuite(s string) (string, string, error) {
	var uri, suite string
	for _, line := range strings.Split(s, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "uris":
			uri = firstField(val)
		case "suites":
			suite = firstField(val)
		}
	}
	if uri == "" || suite == "" {
		return "", "", fmt.Errorf("deb822 stanza missing URIs or Suites")
	}
	return uri, suite, nil
}

// oneLineURISuite parses "deb [opt=v opt=v] URI suite [components...]". The
// bracketed option group may itself contain spaces, so the URI cannot be
// picked out by field position alone.
func oneLineURISuite(s string) (string, string, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 || (fields[0] != "deb" && fields[0] != "deb-src") {
		return "", "", fmt.Errorf("not an apt repo line: %q", s)
	}
	fields = fields[1:]

	if len(fields) > 0 && strings.HasPrefix(fields[0], "[") {
		for len(fields) > 0 {
			last := strings.HasSuffix(fields[0], "]")
			fields = fields[1:]
			if last {
				break
			}
		}
	}
	if len(fields) < 2 {
		return "", "", fmt.Errorf("apt repo line missing URI or suite: %q", s)
	}
	return fields[0], fields[1], nil
}

func firstField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func isArmoredKey(data []byte) bool {
	return bytes.Contains(data, []byte(armorHeader))
}

func isSubset(want, have []string) bool {
	if len(want) == 0 {
		return false
	}
	set := make(map[string]bool, len(have))
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

// fetchTemp downloads a URL to a scratch file as the invoking user and
// returns its path plus a cleanup func. Nothing here needs root: the download
// only becomes a root-owned file later, via writeFileAsRoot.
func fetchTemp(ctx context.Context, url string) (string, func(), error) {
	path, cleanup, err := writeTemp("dfinstall-fetch-*", nil)
	if err != nil {
		return "", nil, err
	}
	if err := exec.CommandContext(ctx, "curl", "-fsSL", "-o", path, url).Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	return path, cleanup, nil
}

func writeTemp(pattern string, data []byte) (string, func(), error) {
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, fmt.Errorf("create temp file: %w", err)
	}
	path := tmp.Name()
	cleanup := func() { os.Remove(path) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return "", nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close temp file: %w", err)
	}
	return path, cleanup, nil
}
