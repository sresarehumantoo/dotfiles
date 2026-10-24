package tests

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points HOME at a scratch directory and clears the XDG variables, so
// a test that forgets to isolate itself writes there instead of the developer's
// real config. A test that sets HOME gets XDG paths derived from it.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "dfinstall-tests-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	for _, v := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		os.Unsetenv(v)
	}
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
