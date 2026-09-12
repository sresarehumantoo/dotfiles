package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The unit tests next door cover the parsing in isolation. What they cannot
// cover is the part that actually broke a machine: whether gpgv is being
// invoked in a way that gives a truthful answer about a real repo. Pointing
// gpgv straight at an armored multi-certificate keyring, for instance, exits
// nonzero even when the signature is good — which would report every repo as
// needing a key refresh, on every converge.
//
// These talk to upstream, so they are opt-in:
//
//	DF_NET_TESTS=1 go test ./src/modules/ -run Live
func requireNetTests(t *testing.T) {
	t.Helper()
	if os.Getenv("DF_NET_TESTS") != "1" {
		t.Skip("network test; set DF_NET_TESTS=1 to run")
	}
	for _, bin := range []string{"curl", "gpg", "gpgv"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
}

// fetchKey downloads an upstream key to a scratch file for the tests below.
func fetchKey(t *testing.T, ctx context.Context, url string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "key.asc")
	if err := exec.CommandContext(ctx, "curl", "-fsSL", "-o", dst, url).Run(); err != nil {
		t.Skipf("could not fetch %s: %v", url, err)
	}
	return dst
}

func TestLiveKeyVerifiesRepo(t *testing.T) {
	requireNetTests(t)
	ctx := context.Background()

	repos := []struct {
		name   string
		keyURL string
		repo   string
	}{
		{
			name:   "hashicorp",
			keyURL: "https://apt.releases.hashicorp.com/gpg",
			repo:   "deb [arch=amd64 signed-by=/dev/null] https://apt.releases.hashicorp.com trixie main",
		},
		{
			name:   "docker",
			keyURL: "https://download.docker.com/linux/debian/gpg",
			repo: `Types: deb
URIs: https://download.docker.com/linux/debian
Suites: trixie
Components: stable
Signed-By: /dev/null`,
		},
	}

	for _, r := range repos {
		t.Run(r.name, func(t *testing.T) {
			url, err := aptInReleaseURL(r.repo)
			if err != nil {
				t.Fatalf("deriving InRelease URL: %v", err)
			}
			inRelease, cleanup, err := fetchTemp(ctx, url)
			if err != nil {
				t.Skipf("could not fetch %s: %v", url, err)
			}
			defer cleanup()

			key := fetchKey(t, ctx, r.keyURL)
			if !gpgvVerify(ctx, key, inRelease) {
				t.Errorf("upstream key from %s does not verify %s", r.keyURL, url)
			}

			// A keyring that cannot verify must come back false, or a
			// rotation is never detected.
			bogus := filepath.Join(t.TempDir(), "empty.asc")
			if err := os.WriteFile(bogus, []byte(armorHeader+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if gpgvVerify(ctx, bogus, inRelease) {
				t.Error("an empty keyring reported a good signature")
			}
			if gpgvVerify(ctx, filepath.Join(t.TempDir(), "absent.asc"), inRelease) {
				t.Error("a missing keyring reported a good signature")
			}
		})
	}
}

// TestLiveMergedKeyringVerifies is the regression guard for the bug that
// motivated all of this: a keyring holding both the outgoing and incoming
// certificate must still verify, and gpgv must be able to read it.
func TestLiveMergedKeyringVerifies(t *testing.T) {
	requireNetTests(t)
	ctx := context.Background()

	url, err := aptInReleaseURL("deb https://apt.releases.hashicorp.com trixie main")
	if err != nil {
		t.Fatal(err)
	}
	inRelease, cleanup, err := fetchTemp(ctx, url)
	if err != nil {
		t.Skipf("could not fetch %s: %v", url, err)
	}
	defer cleanup()

	current := fetchKey(t, ctx, "https://apt.releases.hashicorp.com/gpg")

	// Stand in for a superseded certificate with any unrelated key. Docker's
	// is convenient: armored like the real thing, so the merge exercises the
	// concatenated-armor path, and certainly not what signs the HashiCorp
	// repo.
	superseded := fetchKey(t, ctx, "https://download.docker.com/linux/debian/gpg")
	if gpgvVerify(ctx, superseded, inRelease) {
		t.Fatal("stand-in key unexpectedly verifies the repo; test is meaningless")
	}

	merged, err := mergeKeyrings(ctx, superseded, current)
	if err != nil {
		t.Fatalf("merging keyrings: %v", err)
	}
	mergedPath := filepath.Join(t.TempDir(), "merged.asc")
	if err := os.WriteFile(mergedPath, merged, 0o644); err != nil {
		t.Fatal(err)
	}

	if !gpgvVerify(ctx, mergedPath, inRelease) {
		t.Error("merged keyring does not verify the repo")
	}

	// Merging the same key twice must not keep growing the file.
	again, err := mergeKeyrings(ctx, mergedPath, current)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if len(again) != len(merged) {
		t.Errorf("re-merging an already-present key changed the keyring: %d -> %d bytes",
			len(merged), len(again))
	}
}
