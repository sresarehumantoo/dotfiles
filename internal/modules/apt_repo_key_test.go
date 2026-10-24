package modules

import (
	"testing"
	"time"
)

func TestAptInReleaseURL(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		want    string
		wantErr bool
	}{
		{
			name: "one-line hashicorp",
			repo: "deb [arch=amd64 signed-by=/usr/share/keyrings/hashicorp-archive-keyring.asc] https://apt.releases.hashicorp.com trixie main",
			want: "https://apt.releases.hashicorp.com/dists/trixie/InRelease",
		},
		{
			name: "deb822 docker",
			repo: `Types: deb
URIs: https://download.docker.com/linux/debian
Suites: trixie
Components: stable
Architectures: amd64
Signed-By: /etc/apt/keyrings/docker.asc`,
			want: "https://download.docker.com/linux/debian/dists/trixie/InRelease",
		},
		{
			// The option group can carry spaces inside the brackets, so the
			// URI is not simply the third field.
			name: "one-line with spaced option group",
			repo: "deb [ signed-by=/usr/share/keyrings/mongodb-server-8.0.gpg ] http://repo.mongodb.org/apt/debian bookworm/mongodb-org/8.0 main",
			want: "http://repo.mongodb.org/apt/debian/dists/bookworm/mongodb-org/8.0/InRelease",
		},
		{
			name: "one-line without option group",
			repo: "deb http://deb.debian.org/debian trixie main",
			want: "http://deb.debian.org/debian/dists/trixie/InRelease",
		},
		{
			// A suite ending in "/" is apt's flat-repo form: no dists/. "./"
			// is a relative path meaning the repo root, not a directory.
			name: "flat repo at root",
			repo: "deb [arch=amd64] https://example.test/repo ./",
			want: "https://example.test/repo/InRelease",
		},
		{
			name: "flat repo in a subdirectory",
			repo: "deb https://example.test/repo stable/",
			want: "https://example.test/repo/stable/InRelease",
		},
		{
			name: "trailing slash on URI is not doubled",
			repo: "deb https://example.test/repo/ trixie main",
			want: "https://example.test/repo/dists/trixie/InRelease",
		},
		{
			name:    "not a repo line",
			repo:    "# a comment",
			wantErr: true,
		},
		{
			name:    "missing suite",
			repo:    "deb https://example.test/repo",
			wantErr: true,
		},
		{
			name:    "deb822 missing Suites",
			repo:    "Types: deb\nURIs: https://example.test/repo",
			wantErr: true,
		},
		{
			name:    "empty",
			repo:    "   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := aptInReleaseURL(tt.repo)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePrimaryFingerprints(t *testing.T) {
	// Trimmed `gpg --show-keys --with-colons` output for the HashiCorp
	// packaging key: one primary with one signing subkey. Only the primary
	// fingerprint should come back, or a rotation is misdetected as a
	// no-op because the subkey happens to match.
	colons := `pub:-:4096:1:AA16FCBCA621E701:1673378787:1831058877::-:::scSC::::::23::0:
fpr:::::::::798AEC654E5C15428C8E42EEAA16FCBCA621E701:
uid:-::::1673378877::43C099891420B0E9E7396113D79A8B10FE6B334D::HashiCorp Security::::::::::0:
sub:-:4096:1:706E668369C085E9:1673378835:1831058835:::::s::::::23:
fpr:::::::::EB0AF5E2994969596F99873E706E668369C085E9:
pub:-:4096:1:FC9CA96ACA026560:1788974540:1946654540::-:::scSC::::::23::0:
fpr:::::::::D55C0D1AC78A8D8126CB631CFC9CA96ACA026560:
uid:-::::1788974540::43C099891420B0E9E7396113D79A8B10FE6B334D::HashiCorp Security::::::::::0:
`

	got := parsePrimaryFingerprints(colons)
	want := []string{
		"798AEC654E5C15428C8E42EEAA16FCBCA621E701",
		"D55C0D1AC78A8D8126CB631CFC9CA96ACA026560",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("fingerprint %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestHasLiveCertificate(t *testing.T) {
	now := time.Unix(1789000000, 0)

	tests := []struct {
		name   string
		colons string
		want   bool
	}{
		{
			name:   "unexpired",
			colons: "pub:-:4096:1:FC9CA96ACA026560:1788974540:1946654540::-:::scSC:\n",
			want:   true,
		},
		{
			name:   "expiry in the past",
			colons: "pub:-:4096:1:AA16FCBCA621E701:1673378787:1673378788::-:::scSC:\n",
			want:   false,
		},
		{
			name:   "no expiry set",
			colons: "pub:-:4096:1:AA16FCBCA621E701:1673378787:::-:::scSC:\n",
			want:   true,
		},
		{
			name:   "flagged expired by gpg",
			colons: "pub:e:4096:1:AA16FCBCA621E701:1673378787:1946654540::-:::scSC:\n",
			want:   false,
		},
		{
			name:   "revoked",
			colons: "pub:r:4096:1:AA16FCBCA621E701:1673378787:1946654540::-:::scSC:\n",
			want:   false,
		},
		{
			name: "one dead cert, one live",
			colons: "pub:r:4096:1:AA16FCBCA621E701:1673378787:1946654540::-:::scSC:\n" +
				"pub:-:4096:1:FC9CA96ACA026560:1788974540:1946654540::-:::scSC:\n",
			want: true,
		},
		{
			name:   "no pub records",
			colons: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasLiveCertificate(tt.colons, now); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSubset(t *testing.T) {
	have := []string{"A", "B"}

	// An empty incoming set must not count as "already present": that would
	// silently skip installing a key gpg failed to parse.
	if isSubset(nil, have) {
		t.Error("empty want should not be a subset")
	}
	if !isSubset([]string{"A"}, have) {
		t.Error("A should be a subset of {A,B}")
	}
	if isSubset([]string{"A", "C"}, have) {
		t.Error("{A,C} should not be a subset of {A,B}")
	}
}

func TestIsArmoredKey(t *testing.T) {
	if !isArmoredKey([]byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\nmQINBA...\n")) {
		t.Error("armored key not detected")
	}
	if isArmoredKey([]byte{0x99, 0x01, 0x0d, 0x04}) {
		t.Error("binary keyring detected as armored")
	}
}
