package modules

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v3/internal/core"
)

// fakeGit puts a git on PATH (and nothing else) that answers from the env
// vars below, so the probe can be driven through every state with no network
// and without reading the real machine's git config.
const fakeGitScript = `#!/bin/sh
case "$1" in
--version)
	[ -n "$FAKE_GIT_EXEC_RC" ] && exit "$FAKE_GIT_EXEC_RC"
	echo "git version 2.47.3" ;;
config)
	[ -n "$FAKE_GIT_CONFIG" ] || exit 1
	printf '%s\n' "$FAKE_GIT_CONFIG" ;;
ls-remote)
	if [ "$2" = "--get-url" ]; then echo "${FAKE_GIT_GETURL:-$3}"; exit 0; fi ;;
-c)
	[ -n "$FAKE_GIT_LSREMOTE_RC" ] && { echo "fatal: unable to access" >&2; exit "$FAKE_GIT_LSREMOTE_RC"; }
	printf 'abc\trefs/heads/main\n' ;;
esac
`

func fakeGit(t *testing.T, env map[string]string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(fakeGitScript), 0o755); err != nil {
		t.Fatal(err)
	}
	// sh itself must stay reachable for the script's shebang.
	t.Setenv("PATH", dir+":/bin:/usr/bin")
	for _, k := range []string{"FAKE_GIT_EXEC_RC", "FAKE_GIT_CONFIG", "FAKE_GIT_GETURL",
		"FAKE_GIT_LSREMOTE_RC", "GIT_ALLOW_PROTOCOL", "GIT_PROTOCOL_FROM_USER"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

const ghTarget = "https://github.com/ohmyzsh/ohmyzsh.git"

func TestGitAccessLocal(t *testing.T) {
	cases := []struct {
		name   string
		env    map[string]string
		state  gitState
		reason string
	}{
		{"ok", nil, gitOK, ""},
		{"blocked exec", map[string]string{"FAKE_GIT_EXEC_RC": "126"}, gitBlockedExec, "does not run"},
		{"wrapper with no git behind it", map[string]string{"FAKE_GIT_EXEC_RC": "127"}, gitAbsent, "not on PATH"},
		{"https never", map[string]string{"FAKE_GIT_CONFIG": "protocol.https.allow never"},
			gitDenied, "protocol.https.allow=never"},
		{"protocol.allow never", map[string]string{"FAKE_GIT_CONFIG": "protocol.allow never"},
			gitDenied, "protocol.allow=never"},
		{"https always beats protocol.allow never",
			map[string]string{"FAKE_GIT_CONFIG": "protocol.allow never\nprotocol.https.allow always"}, gitOK, ""},
		{"user with from-user 0",
			map[string]string{"FAKE_GIT_CONFIG": "protocol.https.allow user", "GIT_PROTOCOL_FROM_USER": "0"},
			gitDenied, "GIT_PROTOCOL_FROM_USER=0"},
		{"user allowed by default", map[string]string{"FAKE_GIT_CONFIG": "protocol.https.allow user"}, gitOK, ""},
		{"allow-list without https", map[string]string{"GIT_ALLOW_PROTOCOL": "ssh:file"},
			gitDenied, "GIT_ALLOW_PROTOCOL=ssh:file"},
		{"allow-list with https", map[string]string{"GIT_ALLOW_PROTOCOL": "file:https"}, gitOK, ""},
		{"mirror", map[string]string{"FAKE_GIT_GETURL": "https://user:secret@git.corp.example/mirror/ohmyzsh.git"},
			gitViaMirror, "https://git.corp.example/mirror/ohmyzsh.git"},
		// The policy applies to the protocol git will actually use.
		{"ssh mirror with ssh denied", map[string]string{
			"FAKE_GIT_GETURL": "git@git.corp.example:mirror/ohmyzsh.git",
			"FAKE_GIT_CONFIG": "protocol.ssh.allow never"}, gitDenied, "ssh is denied"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeGit(t, c.env)
			a := gitAccessLocal(context.Background(), ghTarget)
			if a.State != c.state {
				t.Fatalf("state = %s (%s), want %s", a.State, a.Reason, c.state)
			}
			if !strings.Contains(a.Reason, c.reason) {
				t.Errorf("reason %q does not mention %q", a.Reason, c.reason)
			}
			if strings.Contains(a.Reason, "secret") {
				t.Errorf("reason leaks credentials: %q", a.Reason)
			}
		})
	}
}

func TestGitAccessAbsent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if a := gitAccessLocal(context.Background(), ghTarget); a.State != gitAbsent {
		t.Fatalf("state = %s, want %s", a.State, gitAbsent)
	}
}

func TestGitReachable(t *testing.T) {
	fakeGit(t, nil)
	if a := gitReachable(context.Background(), gitAccessLocal(context.Background(), ghTarget)); a.State != gitOK {
		t.Errorf("reachable: state = %s (%s)", a.State, a.Reason)
	}

	fakeGit(t, map[string]string{"FAKE_GIT_LSREMOTE_RC": "128"})
	a := gitReachable(context.Background(), gitAccessLocal(context.Background(), ghTarget))
	if a.State != gitUnreachable || !strings.Contains(a.Reason, "unable to access") {
		t.Errorf("unreachable: state = %s, reason %q", a.State, a.Reason)
	}

	// A denied result is never upgraded to a network attempt.
	fakeGit(t, map[string]string{"FAKE_GIT_CONFIG": "protocol.https.allow never", "FAKE_GIT_LSREMOTE_RC": "128"})
	if a := gitReachable(context.Background(), gitAccessLocal(context.Background(), ghTarget)); a.State != gitDenied {
		t.Errorf("denied: state = %s", a.State)
	}
}

func TestGitAccessDoctorNeverFails(t *testing.T) {
	fakeGit(t, map[string]string{"FAKE_GIT_CONFIG": "protocol.https.allow never\nhttp.proxy http://u:p@proxy.corp:3128"})
	r := gitAccessDoctorResult()
	if !r.OK || !strings.Contains(r.Detail, "denied-by-policy") || !strings.Contains(r.Detail, "proxy.corp:3128") {
		t.Errorf("doctor = %+v", r)
	}
	if strings.Contains(r.Detail, "u:p@") {
		t.Errorf("doctor leaks proxy credentials: %q", r.Detail)
	}
}

func TestGitProtocol(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/x/y.git": "https",
		"ssh://git@host/x.git":       "ssh",
		"git@host:x/y.git":           "ssh",
		"/srv/git/x.git":             "file",
		"file:///srv/git/x.git":      "file",
	} {
		if got := gitProtocol(in); got != want {
			t.Errorf("gitProtocol(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOmzSkipsWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := OmzModule{}.Install(context.Background())
	var skip *core.SkipError
	if !errors.As(err, &skip) || !strings.Contains(skip.Reason, "not on PATH") {
		t.Fatalf("Install = %v, want a SkipError naming the missing git", err)
	}
}

// A personal rewrite of GitHub https to ssh is common and is not a mirror.
func TestGitAccessDoctorRewriteWording(t *testing.T) {
	fakeGit(t, map[string]string{"FAKE_GIT_GETURL": "git@github.com:"})
	r := gitAccessDoctorResult()
	if !strings.Contains(r.Detail, "rewritten by insteadOf to git@github.com:") || strings.Contains(r.Detail, "mirror") {
		t.Errorf("doctor = %q", r.Detail)
	}
}
