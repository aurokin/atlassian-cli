// Package browser opens a URL in the user's default web browser. It is the
// single place that knows the per-platform "open this URL" incantation.
package browser

import (
	"net/url"
	"os/exec"
	"runtime"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// runner starts an external command. It is a package variable so tests can
// substitute a recorder instead of spawning a real process.
var runner = func(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

// Open opens rawURL in the platform's default browser. It refuses any URL
// whose scheme is not http(s), so a crafted value can never be handed to the
// operating system's URL handler.
func Open(rawURL string) error {
	return openWith(runtime.GOOS, rawURL)
}

// openWith is Open with the platform made explicit so tests can exercise the
// per-platform branches, including an unsupported platform.
func openWith(goos, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return apperr.InvalidInput("refusing to open a non-http(s) URL")
	}
	name, args := openCommand(goos, rawURL)
	if name == "" {
		return apperr.New("unsupported_platform", "opening a browser is not supported on "+goos)
	}
	if err := runner(name, args...); err != nil {
		return apperr.New("browser_failed", "could not open a browser: "+err.Error())
	}
	return nil
}

// openCommand returns the command and arguments that open a URL on goos. An
// empty name means goos has no known opener.
func openCommand(goos, rawURL string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{rawURL}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", rawURL}
	case "linux":
		return "xdg-open", []string{rawURL}
	default:
		return "", nil
	}
}
