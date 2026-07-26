package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

const validRegistryJSON = `{"version":1,"tools":[
 {"name":"jq","description":"d","category":"c","method":"apt","binary":"jq","package":"jq"}
]}`

func TestFetchRegistry_OK(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(validRegistryJSON))
	}))
	defer srv.Close()

	reg, err := core.FetchRegistry(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchRegistry: %v", err)
	}
	if len(reg.Tools) != 1 || reg.Tools[0].Name != "jq" {
		t.Fatalf("unexpected registry: %+v", reg)
	}
}

// A non-200 used to surface as curl's "exit status 22" with no indication of
// what the server actually said.
func TestFetchRegistry_ReportsHTTPStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := core.FetchRegistry(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q should name the HTTP status", err)
	}
}

// The whole point of the context: a server that never responds must not hang
// dfinstall forever. Previously this shelled out to curl with no timeout.
func TestFetchRegistry_CancellableWhileServerHangs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // never respond until the test is done
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	done := make(chan error, 1)
	go func() {
		_, err := core.FetchRegistry(ctx, srv.URL)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error when the context is cancelled")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("FetchRegistry ignored context cancellation and hung")
	}
}

// A registry endpoint that streams forever must not exhaust memory.
func TestFetchRegistry_RejectsOversizedBody(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	chunk := strings.Repeat("a", 1<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 16; i++ {
			if _, err := w.Write([]byte(chunk)); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	_, err := core.FetchRegistry(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for an oversized registry")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error %q should report the size limit", err)
	}
}
