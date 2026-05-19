// Package secrets stores and retrieves a per-site credential token outside the
// plaintext config file. The preferred backend is the operating system
// keychain (via go-keyring); when no keychain is available the fallback is a
// 0600 credentials file. config.json only ever records an indirect token_ref
// pointing at one of these backends, never a raw token.
package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// keyringService is the service name every credential is stored under in the
// OS keychain; the keychain account is the site name.
const keyringService = "atlassian-cli"

// credentialsVersion is the schema version of the fallback credentials file.
const credentialsVersion = 1

// Backend identifiers. A profile's token_ref records one of these verbatim so
// a later read knows which backend to consult.
const (
	BackendKeyring = "keyring"
	BackendFile    = "file"
)

// Store persists one site's credential token. Implementations key on the site
// name; the token value is never returned to or logged by anything but the
// request-signing path.
type Store interface {
	// Name is the backend identifier recorded as a profile's token_ref.
	Name() string
	// Set stores token for site, replacing any existing value.
	Set(site, token string) error
	// Get returns the stored token, or a structured token_unavailable error
	// when no token is stored for site.
	Get(site string) (string, error)
	// Delete removes any stored token for site. Deleting an absent token is
	// not an error.
	Delete(site string) error
}

// SaveResult reports where Save placed a token.
type SaveResult struct {
	// Backend is the token_ref to record: BackendKeyring or BackendFile.
	Backend string
	// FellBack is true when the OS keychain was unusable and the 0600 file
	// backend was used instead.
	FellBack bool
	// KeyringErr explains why the keychain was unusable; set only when
	// FellBack is true, for the caller to surface as a warning.
	KeyringErr error
}

// Save stores token for site, preferring the OS keychain and falling back to a
// 0600 credentials file at credPath when the keychain cannot be written. The
// caller records SaveResult.Backend as the profile's token_ref and, when
// SaveResult.FellBack is set, warns the user that the token is not
// keychain-protected.
func Save(credPath, site, token string) (SaveResult, error) {
	ks := keyringStore{}
	keyringErr := ks.Set(site, token)
	if keyringErr == nil {
		return SaveResult{Backend: ks.Name()}, nil
	}
	fs := fileStore{path: credPath}
	if err := fs.Set(site, token); err != nil {
		return SaveResult{}, err
	}
	return SaveResult{Backend: fs.Name(), FellBack: true, KeyringErr: keyringErr}, nil
}

// ForRef returns the Store backing a recorded token_ref. credPath locates the
// fallback credentials file and is consulted only for the file backend.
func ForRef(ref, credPath string) (Store, error) {
	switch ref {
	case BackendKeyring:
		return keyringStore{}, nil
	case BackendFile:
		return fileStore{path: credPath}, nil
	default:
		return nil, apperr.New("token_unavailable",
			fmt.Sprintf("unsupported stored-credential backend %q", ref))
	}
}

// keyringStore is the OS-keychain backend.
type keyringStore struct{}

func (keyringStore) Name() string { return BackendKeyring }

func (keyringStore) Set(site, token string) error {
	if err := keyring.Set(keyringService, site, token); err != nil {
		return fmt.Errorf("keyring: store credential: %w", err)
	}
	return nil
}

func (keyringStore) Get(site string) (string, error) {
	v, err := keyring.Get(keyringService, site)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", apperr.New("token_unavailable",
			fmt.Sprintf("no stored credential for site %q in the OS keychain", site))
	}
	if err != nil {
		return "", apperr.New("credential_read_failed",
			"could not read the credential from the OS keychain: "+err.Error())
	}
	return v, nil
}

func (keyringStore) Delete(site string) error {
	err := keyring.Delete(keyringService, site)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("keyring: delete credential: %w", err)
}

// fileStore is the 0600-file fallback backend, used when no keychain exists.
type fileStore struct{ path string }

func (fileStore) Name() string { return BackendFile }

// credentialsDoc is the on-disk shape of the fallback credentials file.
type credentialsDoc struct {
	Version int               `json:"version"`
	Tokens  map[string]string `json:"tokens"`
}

func (f fileStore) load() (credentialsDoc, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return credentialsDoc{Version: credentialsVersion, Tokens: map[string]string{}}, nil
	}
	if err != nil {
		return credentialsDoc{}, apperr.New("credential_read_failed",
			fmt.Sprintf("could not read the credentials file %s: %v", f.path, err))
	}
	var doc credentialsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return credentialsDoc{}, apperr.New("invalid_config",
			fmt.Sprintf("credentials file %s is not valid JSON: %v", f.path, err))
	}
	if doc.Tokens == nil {
		doc.Tokens = map[string]string{}
	}
	return doc, nil
}

// save writes doc to f.path as 0600 JSON. The directory is created 0700 and
// the write is atomic: a temp file in the same directory is renamed over the
// target, so a crash or concurrent run never observes a partial file and the
// rename replaces a symlink rather than following it.
func (f fileStore) save(doc credentialsDoc) error {
	doc.Version = credentialsVersion
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("secrets: marshal: %w", err)
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("secrets: create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".credentials-*.json")
	if err != nil {
		return fmt.Errorf("secrets: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename succeeds

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secrets: set permissions on %s: %w", tmpName, err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secrets: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("secrets: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, f.path); err != nil {
		return fmt.Errorf("secrets: replace %s: %w", f.path, err)
	}
	return nil
}

func (f fileStore) Set(site, token string) error {
	doc, err := f.load()
	if err != nil {
		return err
	}
	doc.Tokens[site] = token
	return f.save(doc)
}

func (f fileStore) Get(site string) (string, error) {
	doc, err := f.load()
	if err != nil {
		return "", err
	}
	token := doc.Tokens[site]
	if token == "" {
		return "", apperr.New("token_unavailable",
			fmt.Sprintf("no stored credential for site %q in %s", site, f.path))
	}
	return token, nil
}

func (f fileStore) Delete(site string) error {
	doc, err := f.load()
	if err != nil {
		return err
	}
	if _, ok := doc.Tokens[site]; !ok {
		return nil
	}
	delete(doc.Tokens, site)
	return f.save(doc)
}
