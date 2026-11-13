package modules

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v3/internal/core"
)

// What git may do on this machine, read from the machine and never changed.
//
// A corporate machine may have no git, an app-control policy that stops it
// running, a git protocol policy, or an insteadOf rule pointing GitHub at an
// internal mirror. dfinstall reads all of that and picks what the environment
// allows. It never writes git config, and a deny is treated as policy, not as
// something to route around with another transport.

type gitState string

const (
	gitOK          gitState = "ok"
	gitViaMirror   gitState = "ok-via-mirror"
	gitAbsent      gitState = "absent"
	gitBlockedExec gitState = "blocked-exec"
	gitDenied      gitState = "denied-by-policy"
	// gitUnreachable is the only state that may be transient.
	gitUnreachable gitState = "unreachable"
)

type gitAccess struct {
	State gitState
	// Reason says why, for every state but gitOK.
	Reason  string
	Version string
	// Target is the URL asked about; URL is what git will fetch after its
	// insteadOf rules: a mirror, or ssh in place of https.
	Target, URL string
	Proxy       string
	CAInfo      string
}

func (a gitAccess) usable() bool { return a.State == gitOK || a.State == gitViaMirror }

// gitAccessLocal classifies what git may do for target without touching the
// network, so doctor can call it.
func gitAccessLocal(ctx context.Context, target string) gitAccess {
	a := gitAccess{Target: target, URL: target}
	if _, err := exec.LookPath("git"); err != nil {
		a.State, a.Reason = gitAbsent, "git is not on PATH"
		return a
	}
	// EDR or app control can leave a binary on disk that cannot execute.
	out, err := runProbe(ctx, "git", "--version")
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee) && ee.ExitCode() == 127:
		// The shell's "command not found": the devtools git wrapper, with no
		// git behind it to run.
		a.State, a.Reason = gitAbsent, "git is not on PATH"
		return a
	case err != nil:
		a.State, a.Reason = gitBlockedExec, fmt.Sprintf("git is installed but does not run: %v", err)
		return a
	}
	a.Version = strings.TrimPrefix(strings.TrimSpace(string(out)), "git version ")

	cfg := gitConfigValues(ctx)
	a.Proxy, a.CAInfo = redactURL(cfg["http.proxy"]), cfg["http.sslcainfo"]
	if out, err := runProbe(ctx, "git", "ls-remote", "--get-url", target); err == nil {
		if u := strings.TrimSpace(string(out)); u != "" {
			a.URL = u
		}
	}

	proto := gitProtocol(a.URL)
	if reason, denied := gitProtocolDenied(proto, cfg); denied {
		a.State, a.Reason = gitDenied, reason
		return a
	}
	a.State = gitOK
	if a.URL != target {
		a.State, a.Reason = gitViaMirror, "insteadOf rewrites it to "+redactURL(a.URL)
	}
	return a
}

// gitReachable adds the network layer to a usable local result: it lists the
// target's heads. It must never wait on a prompt, so terminal prompts,
// credential helpers and interactive ssh are all off.
func gitReachable(ctx context.Context, a gitAccess) gitAccess {
	if !a.usable() {
		return a
	}
	ctx, cancel := context.WithTimeout(ctx, core.ProbeTimeout)
	defer cancel()
	args := []string{"-c", "credential.helper=", "ls-remote", "--heads", a.Target}
	cmd := exec.CommandContext(ctx, "git", args...)
	// runNetProbe takes no environment, and these are what keep the probe
	// from hanging on a credential or ssh passphrase prompt.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes",
	)
	if _, err := cmd.Output(); err != nil {
		a.State, a.Reason = gitUnreachable, probeErr("git", args, err).Error()
	}
	return a
}

// gitConfigValues reads the config keys the probe cares about. Keys come back
// lowercased by git. A missing key, or no git config at all, is an empty map.
func gitConfigValues(ctx context.Context) map[string]string {
	vals := map[string]string{}
	out, err := runProbe(ctx, "git", "config", "--get-regexp",
		`^(protocol\.allow|protocol\..*\.allow|http\.proxy|http\.sslcainfo)$`)
	if err != nil {
		return vals
	}
	for _, line := range strings.Split(string(out), "\n") {
		k, v, _ := strings.Cut(strings.TrimSpace(line), " ")
		if k != "" {
			vals[k] = v
		}
	}
	return vals
}

// gitProtocol names the transport git uses for u, as git's protocol.*.allow
// keys spell it.
func gitProtocol(u string) string {
	if pu, err := url.Parse(u); err == nil && pu.Scheme != "" && strings.Contains(u, "://") {
		return strings.ToLower(pu.Scheme)
	}
	// scp-like user@host:path is ssh; anything else is a local path.
	if i := strings.Index(u, ":"); i > 0 && !strings.Contains(u[:i], "/") {
		return "ssh"
	}
	return "file"
}

// gitProtocolDenied applies git's own rules: GIT_ALLOW_PROTOCOL is an
// allow-list when set; otherwise protocol.<name>.allow, then protocol.allow,
// then git's default (always for http, https, git and ssh, user for the rest).
// "user" allows a command the user ran unless GIT_PROTOCOL_FROM_USER is 0.
func gitProtocolDenied(proto string, cfg map[string]string) (string, bool) {
	if list, ok := os.LookupEnv("GIT_ALLOW_PROTOCOL"); ok {
		for _, p := range strings.Split(list, ":") {
			if p == proto {
				return "", false
			}
		}
		return fmt.Sprintf("GIT_ALLOW_PROTOCOL=%s does not include %s", list, proto), true
	}
	key, policy := "protocol."+proto+".allow", cfg["protocol."+proto+".allow"]
	if policy == "" {
		key, policy = "protocol.allow", cfg["protocol.allow"]
	}
	if policy == "" {
		switch proto {
		case "http", "https", "git", "ssh":
			return "", false
		}
		key, policy = "git's default for "+proto, "user"
	}
	switch policy {
	case "never":
		return fmt.Sprintf("%s is denied by %s=never", proto, key), true
	case "user":
		if os.Getenv("GIT_PROTOCOL_FROM_USER") == "0" {
			return fmt.Sprintf("%s is denied by %s=user with GIT_PROTOCOL_FROM_USER=0", proto, key), true
		}
	}
	return "", false
}

// gitAccessDoctorResult reports the local layers only, so doctor stays offline.
// It never fails: a machine without git is a configuration, not a fault.
func gitAccessDoctorResult() DoctorResult {
	// No context to inherit: doctor runs as a synchronous report.
	a := gitAccessLocal(context.Background(), "https://github.com/")
	var detail string
	switch a.State {
	case gitOK:
		detail = "git " + a.Version + ", https allowed"
	case gitViaMirror:
		detail = fmt.Sprintf("git %s, https://github.com/ rewritten by insteadOf to %s", a.Version, redactURL(a.URL))
	default:
		detail = string(a.State) + ": " + a.Reason
	}
	if a.Proxy != "" {
		detail += "; http.proxy " + a.Proxy
	}
	if a.CAInfo != "" {
		detail += "; http.sslCAInfo " + a.CAInfo
	}
	return DoctorResult{Name: "git access", OK: true, Detail: detail}
}
