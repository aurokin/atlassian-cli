package browser

import (
	"errors"
	"testing"
)

// swapRunner replaces the package runner and returns a restore func.
func swapRunner(fn func(string, ...string) error) func() {
	prev := runner
	runner = fn
	return func() { runner = prev }
}

func TestOpenInvokesRunnerWithURL(t *testing.T) {
	var gotName string
	var gotArgs []string
	defer swapRunner(func(name string, args ...string) error {
		gotName, gotArgs = name, args
		return nil
	})()

	const target = "https://x.atlassian.net/browse/PROJ-1"
	if err := Open(target); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if gotName == "" {
		t.Fatal("Open did not invoke the runner")
	}
	if len(gotArgs) == 0 || gotArgs[len(gotArgs)-1] != target {
		t.Fatalf("runner args = %v, want the URL last", gotArgs)
	}
}

func TestOpenRejectsNonHTTPURL(t *testing.T) {
	called := false
	defer swapRunner(func(string, ...string) error { called = true; return nil })()

	for _, bad := range []string{"file:///etc/passwd", "javascript:alert(1)", "ftp://x/y"} {
		if err := Open(bad); err == nil {
			t.Errorf("Open(%q) returned no error", bad)
		}
	}
	if called {
		t.Fatal("Open invoked the runner for a non-http(s) URL")
	}
}

func TestOpenMapsRunnerFailureToError(t *testing.T) {
	defer swapRunner(func(string, ...string) error { return errors.New("boom") })()
	if err := Open("https://x.atlassian.net/browse/PROJ-1"); err == nil {
		t.Fatal("Open returned no error when the runner failed")
	}
}

func TestOpenCommandPerPlatform(t *testing.T) {
	cases := []struct {
		goos string
		name string
	}{
		{"darwin", "open"},
		{"linux", "xdg-open"},
		{"windows", "rundll32"},
		{"plan9", ""},
	}
	for _, tc := range cases {
		name, args := openCommand(tc.goos, "https://x")
		if name != tc.name {
			t.Errorf("openCommand(%q) name = %q, want %q", tc.goos, name, tc.name)
		}
		if tc.name != "" && (len(args) == 0 || args[len(args)-1] != "https://x") {
			t.Errorf("openCommand(%q) args = %v, want the URL last", tc.goos, args)
		}
	}
}
