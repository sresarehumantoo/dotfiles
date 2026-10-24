package modules

// In-package: the signature and archive checks behind installTerraformRelease
// are unexported, and are what make a user-local terraform as trustworthy as
// the apt repo it replaced.

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// gpg --status-fd output for terraform_1.16.5_SHA256SUMS.72D7468F.sig, signed
// by a subkey of HashiCorp's primary key.
const hashicorpVerifyStatus = `[GNUPG:] NEWSIG
[GNUPG:] KEY_CONSIDERED C874011F0AB405110D02105534365D9472D7468F 0
[GNUPG:] GOODSIG C820C6D5CD27AB87 HashiCorp Security (hashicorp.com/security) <security@hashicorp.com>
[GNUPG:] VALIDSIG 374EC75B485913604A831CC7C820C6D5CD27AB87 2026-10-02 1790941082 0 4 0 1 8 00 C874011F0AB405110D02105534365D9472D7468F
`

func TestValidSigFrom(t *testing.T) {
	if !validSigFrom(hashicorpVerifyStatus, hashicorpKeyFingerprint) {
		t.Fatal("HashiCorp's real verify output was rejected")
	}
	other := "[GNUPG:] VALIDSIG AAAA 2026-10-02 1 0 4 0 1 8 00 0123456789ABCDEF0123456789ABCDEF01234567\n"
	if validSigFrom(other, hashicorpKeyFingerprint) {
		t.Fatal("a valid signature from another key was accepted")
	}
	// The signing subkey is not the pin; only the primary-key field counts.
	subkeyOnly := "[GNUPG:] VALIDSIG C874011F0AB405110D02105534365D9472D7468F 2026-10-02 1 0 4 0 1 8 00 0123456789ABCDEF0123456789ABCDEF01234567\n"
	if validSigFrom(subkeyOnly, hashicorpKeyFingerprint) {
		t.Fatal("matched the signing-key field instead of the primary-key field")
	}
	if validSigFrom("[GNUPG:] BADSIG C820C6D5CD27AB87 HashiCorp\n", hashicorpKeyFingerprint) {
		t.Fatal("a bad signature was accepted")
	}
}

func TestExtractZipFile(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "terraform_1.0.0_linux_amd64.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range map[string]string{"LICENSE.txt": "license", "terraform": "binary"} {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	dst := filepath.Join(dir, "out")
	if err := extractZipFile(zipPath, "terraform", dst); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "binary" {
		t.Fatalf("extracted %q, want the terraform member", got)
	}
	if err := extractZipFile(zipPath, "missing", filepath.Join(dir, "x")); err == nil {
		t.Fatal("expected an error for an absent member")
	}
}
