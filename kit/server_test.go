package kit

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/renniemaharaj/go-api-kit/internal/environment"
)

func TestRunClosesFrontendRootAfterBuildFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("frontend"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FRONTEND_DIR", directory)

	failure := errors.New("application construction failed")
	var root fs.FS
	err := Run(func(fsys fs.FS) (http.Handler, error) {
		root = fsys
		if _, err := fs.ReadFile(root, "index.html"); err != nil {
			t.Fatalf("frontend root was not available during construction: %v", err)
		}

		return nil, failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("startup lost the application error: %v", err)
	}

	file, err := root.Open("index.html")
	if err == nil {
		file.Close()
		t.Fatal("startup left the frontend root open after construction failed")
	}
}

func TestRunBuildsAPIOnlyApplicationsWithoutAFrontendRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("FRONTEND_DIR", "")
	failure := errors.New("stop before listening")
	err := Run(func(fsys fs.FS) (http.Handler, error) {
		if fsys != nil {
			t.Fatal("API-only application received a frontend root")
		}

		return nil, failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("startup lost the application error: %v", err)
	}
}

func TestServerUsesAutomaticEnvironmentDefaultsAndOverrides(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, name := range []string{
		"SERVER_ADDRESS",
		"REQUEST_TIMEOUT",
		"SERVER_READ_HEADER_TIMEOUT",
		"SERVER_READ_TIMEOUT",
		"SERVER_WRITE_TIMEOUT",
		"SERVER_IDLE_TIMEOUT",
	} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}

	values, err := environment.Load(".env")
	if err != nil {
		t.Fatal(err)
	}
	server, err := newServer(http.NotFoundHandler(), values)
	if err != nil {
		t.Fatal(err)
	}
	if server.Addr != "localhost:9100" || server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 20*time.Second || server.WriteTimeout != 20*time.Second || server.IdleTimeout != time.Minute {
		t.Fatalf("unexpected server defaults: %#v", server)
	}

	contents := "SERVER_ADDRESS=localhost:9200\nREQUEST_TIMEOUT=2s\nSERVER_READ_HEADER_TIMEOUT=1s\nSERVER_IDLE_TIMEOUT=30s\n"
	if err := os.WriteFile(".env", []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SERVER_ADDRESS", "localhost:9300")
	values, err = environment.Load(".env")
	if err != nil {
		t.Fatal(err)
	}
	server, err = newServer(http.NotFoundHandler(), values)
	if err != nil {
		t.Fatal(err)
	}
	if server.Addr != "localhost:9300" || server.ReadHeaderTimeout != time.Second || server.ReadTimeout != 7*time.Second || server.WriteTimeout != 7*time.Second || server.IdleTimeout != 30*time.Second {
		t.Fatalf("environment defaults and overrides were not applied: %#v", server)
	}

	for _, timeout := range []string{"invalid", "0s", "-1s"} {
		t.Setenv("SERVER_IDLE_TIMEOUT", timeout)
		values, err := environment.Load(".env")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := newServer(http.NotFoundHandler(), values); err == nil {
			t.Fatalf("invalid timeout %q was accepted", timeout)
		}
	}
}
