package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// TestMain installs the in-memory keyring mock for the whole package so no
// test ever reads from or writes to the real OS keychain.
func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

// credPath returns a credentials-file path inside a fresh temp directory.
func credPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "credentials.json")
}

func TestKeyringStoreHas(t *testing.T) {
	keyring.MockInit()
	var s Store = keyringStore{}
	if has, err := s.Has("absent"); err != nil || has {
		t.Fatalf("Has(absent) = %v, %v; want false, nil", has, err)
	}
	if err := s.Set("work", "tok"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if has, err := s.Has("work"); err != nil || !has {
		t.Fatalf("Has(work) = %v, %v; want true, nil", has, err)
	}
}

func TestKeyringStoreHasReadFailureIsStructured(t *testing.T) {
	keyring.MockInitWithError(errors.New("keychain is locked"))
	t.Cleanup(keyring.MockInit)
	_, err := keyringStore{}.Has("work")
	if err == nil {
		t.Fatal("expected an error when the keychain cannot be read")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestFileStoreHas(t *testing.T) {
	var s Store = fileStore{path: credPath(t)}
	if has, err := s.Has("work"); err != nil || has {
		t.Fatalf("Has(work) on empty store = %v, %v; want false, nil", has, err)
	}
	if err := s.Set("work", "tok"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if has, err := s.Has("work"); err != nil || !has {
		t.Fatalf("Has(work) = %v, %v; want true, nil", has, err)
	}
	if has, _ := s.Has("other"); has {
		t.Fatal("Has(other) = true, want false")
	}
}

func TestKeyringStoreRoundTrip(t *testing.T) {
	keyring.MockInit()
	var s Store = keyringStore{}
	if s.Name() != BackendKeyring {
		t.Fatalf("Name() = %q, want %q", s.Name(), BackendKeyring)
	}
	if err := s.Set("work", "tok-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("work")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "tok-1" {
		t.Fatalf("Get = %q, want tok-1", got)
	}
	if err := s.Delete("work"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("work"); err == nil {
		t.Fatal("Get after Delete returned no error")
	}
}

func TestKeyringStoreGetAbsentIsStructured(t *testing.T) {
	keyring.MockInit()
	_, err := keyringStore{}.Get("never-set")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "token_unavailable" {
		t.Fatalf("error = %v, want a token_unavailable *apperr.Error", err)
	}
}

func TestKeyringStoreDeleteAbsentIsNoError(t *testing.T) {
	keyring.MockInit()
	if err := (keyringStore{}).Delete("never-set"); err != nil {
		t.Fatalf("Delete of absent credential: %v", err)
	}
}

// TestKeyringStoreGetReadFailureIsStructured covers a keychain that fails for
// a reason other than a missing entry (locked, unavailable): the error must be
// distinguishable from a genuine absence so callers do not misreport it.
func TestKeyringStoreGetReadFailureIsStructured(t *testing.T) {
	keyring.MockInitWithError(errors.New("keychain is locked"))
	t.Cleanup(keyring.MockInit)
	_, err := keyringStore{}.Get("work")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "credential_read_failed" {
		t.Fatalf("error = %v, want a credential_read_failed *apperr.Error", err)
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	path := credPath(t)
	var s Store = fileStore{path: path}
	if s.Name() != BackendFile {
		t.Fatalf("Name() = %q, want %q", s.Name(), BackendFile)
	}
	if err := s.Set("work", "tok-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Set("home", "tok-2"); err != nil {
		t.Fatalf("Set second site: %v", err)
	}
	got, err := s.Get("work")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "tok-1" {
		t.Fatalf("Get = %q, want tok-1", got)
	}
	// A second site is unaffected by the first.
	if got, _ := s.Get("home"); got != "tok-2" {
		t.Fatalf("Get home = %q, want tok-2", got)
	}
	if err := s.Delete("work"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("work"); err == nil {
		t.Fatal("Get after Delete returned no error")
	}
	if got, _ := s.Get("home"); got != "tok-2" {
		t.Fatalf("Delete of work disturbed home: got %q", got)
	}
}

func TestFileStoreWritesPrivateFile(t *testing.T) {
	path := credPath(t)
	if err := (fileStore{path: path}).Set("work", "tok"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credentials file mode = %o, want 600", perm)
	}
}

func TestFileStoreGetAbsentIsStructured(t *testing.T) {
	_, err := fileStore{path: credPath(t)}.Get("never-set")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "token_unavailable" {
		t.Fatalf("error = %v, want a token_unavailable *apperr.Error", err)
	}
}

func TestFileStoreDeleteAbsentIsNoError(t *testing.T) {
	if err := (fileStore{path: credPath(t)}).Delete("never-set"); err != nil {
		t.Fatalf("Delete of absent credential: %v", err)
	}
}

func TestFileStoreRejectsMalformedFile(t *testing.T) {
	path := credPath(t)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, err := fileStore{path: path}.Get("work")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "invalid_config" {
		t.Fatalf("error = %v, want an invalid_config *apperr.Error", err)
	}
}

// TestFileStoreReadFailureIsStructured covers a credentials path that exists
// but cannot be read as a file. A directory at the path makes os.ReadFile fail
// with a non-ENOENT error, standing in for a permission or I/O failure; the
// result must be a structured error distinct from a genuine absence.
func TestFileStoreReadFailureIsStructured(t *testing.T) {
	path := credPath(t)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := fileStore{path: path}.Get("work")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "credential_read_failed" {
		t.Fatalf("error = %v, want a credential_read_failed *apperr.Error", err)
	}
}

func TestSavePrefersKeyring(t *testing.T) {
	keyring.MockInit()
	res, err := Save(credPath(t), "work", "tok")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if res.Backend != BackendKeyring || res.FellBack {
		t.Fatalf("Save result = %+v, want keyring backend, no fallback", res)
	}
	got, err := keyringStore{}.Get("work")
	if err != nil || got != "tok" {
		t.Fatalf("keyring after Save = %q, %v; want tok", got, err)
	}
}

func TestSaveFallsBackToFileWhenKeyringFails(t *testing.T) {
	keyring.MockInitWithError(errors.New("no keychain available"))
	t.Cleanup(keyring.MockInit)

	path := credPath(t)
	res, err := Save(path, "work", "tok")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if res.Backend != BackendFile || !res.FellBack {
		t.Fatalf("Save result = %+v, want file backend with fallback", res)
	}
	if res.KeyringErr == nil {
		t.Fatal("Save fell back but reported no keyring error")
	}
	got, err := fileStore{path: path}.Get("work")
	if err != nil || got != "tok" {
		t.Fatalf("file store after fallback Save = %q, %v; want tok", got, err)
	}
}

func TestForRefDispatch(t *testing.T) {
	path := credPath(t)
	tests := []struct {
		ref     string
		want    string
		wantErr bool
	}{
		{BackendKeyring, BackendKeyring, false},
		{BackendFile, BackendFile, false},
		{"env:SOMETHING", "", true},
		{"", "", true},
	}
	for _, tc := range tests {
		s, err := ForRef(tc.ref, path)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ForRef(%q) returned no error", tc.ref)
			}
			continue
		}
		if err != nil {
			t.Errorf("ForRef(%q): %v", tc.ref, err)
			continue
		}
		if s.Name() != tc.want {
			t.Errorf("ForRef(%q).Name() = %q, want %q", tc.ref, s.Name(), tc.want)
		}
	}
}
