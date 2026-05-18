package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestNewHasCurrentVersionAndEmptySites(t *testing.T) {
	c := New()
	if c.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", c.Version, CurrentVersion)
	}
	if c.Sites == nil {
		t.Fatal("Sites is nil, want empty map")
	}
	if len(c.Sites) != 0 {
		t.Errorf("len(Sites) = %d, want 0", len(c.Sites))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	want := New()
	want.Sites["work"] = SiteProfile{
		Product:    "jira",
		Deployment: "cloud",
		BaseURL:    "https://example.atlassian.net",
		TokenStyle: "cloud-classic",
		AuthType:   "api-token-basic",
		Username:   "user@example.com",
		TokenRef:   "env:ATLASSIAN_API_TOKEN",
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("Load of missing file returned error: %v", err)
	}
	if !reflect.DeepEqual(got, New()) {
		t.Fatalf("Load of missing file = %+v, want empty config", got)
	}
}

func TestLoadMalformedFileReturnsStructuredError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load of malformed file returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestSaveUsesRestrictivePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permissions not enforced on Windows")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Save(path, New()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("file mode = %o, want 600", perm)
	}
}

func TestSaveReplacesSymlinkInsteadOfFollowingIt(t *testing.T) {
	dir := t.TempDir()
	realTarget := filepath.Join(dir, "real-target.json")
	if err := os.WriteFile(realTarget, []byte("ORIGINAL"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	link := filepath.Join(dir, "config.json")
	if err := os.Symlink(realTarget, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if err := Save(link, New()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("Save followed the symlink instead of replacing it")
	}
	if got, _ := os.ReadFile(realTarget); string(got) != "ORIGINAL" {
		t.Errorf("Save wrote through the symlink to its target: %q", got)
	}
}

func TestDefaultPathHonorsXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(dir, configDirName, configFileName)
	if got != want {
		t.Fatalf("DefaultPath = %q, want %q", got, want)
	}
}
