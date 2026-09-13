package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/config"
	"github.com/aurokin/atlassian-cli/internal/oauth"
	"github.com/aurokin/atlassian-cli/internal/secrets"
)

// storeOAuthBundle writes a bundle to the (mocked) keychain for site.
func storeOAuthBundle(t *testing.T, site string, b oauth.TokenBundle) {
	t.Helper()
	value, err := b.Marshal()
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	credPath, err := config.CredentialsPath()
	if err != nil {
		t.Fatalf("cred path: %v", err)
	}
	if _, err := secrets.Save(credPath, site, value); err != nil {
		t.Fatalf("store bundle: %v", err)
	}
}

func readOAuthBundle(t *testing.T, site string) oauth.TokenBundle {
	t.Helper()
	credPath, _ := config.CredentialsPath()
	store, _ := secrets.ForRef(secrets.BackendKeyring, credPath)
	value, err := store.Get(site)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	b, err := oauth.ParseBundle(value)
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	return b
}

func oauthProfile() config.SiteProfile {
	return config.SiteProfile{
		Product:    "jira",
		Deployment: "cloud",
		BaseURL:    "https://example.atlassian.net",
		CloudID:    "cloud-123",
		TokenStyle: string(auth.StyleOAuth3LO),
		AuthType:   auth.StyleOAuth3LO.AuthType(),
		TokenRef:   secrets.BackendKeyring,
		ClientID:   "client-abc",
	}
}

func TestOAuthProviderReturnsValidTokenWithoutRefresh(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Token endpoint that fails the test if a refresh is attempted.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("refresh should not happen for a still-valid token")
	}))
	defer srv.Close()
	defer swapOAuthEndpoints(oauth.Endpoints{Token: srv.URL + "/token"})()

	storeOAuthBundle(t, "work", oauth.TokenBundle{
		ClientSecret: "secret", AccessToken: "valid-access", RefreshToken: "r",
		Expiry: time.Now().Add(time.Hour),
	})
	provider, err := oauthCredentialProvider("work", oauthProfile(), nil)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	cred, err := provider(context.Background())
	if err != nil {
		t.Fatalf("provider call: %v", err)
	}
	if cred.Style != auth.StyleOAuth3LO || cred.Token != "valid-access" || cred.CloudID != "cloud-123" {
		t.Fatalf("credential = %+v", cred)
	}
}

func TestOAuthProviderRefreshesExpiredTokenAndPersists(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"new-access","refresh_token":"rotated-refresh","expires_in":3600}`)
	}))
	defer srv.Close()
	defer swapOAuthEndpoints(oauth.Endpoints{Token: srv.URL + "/token"})()

	storeOAuthBundle(t, "work", oauth.TokenBundle{
		ClientSecret: "secret", AccessToken: "old-access", RefreshToken: "old-refresh",
		Expiry: time.Now().Add(-time.Minute),
	})
	provider, err := oauthCredentialProvider("work", oauthProfile(), nil)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	cred, err := provider(context.Background())
	if err != nil {
		t.Fatalf("provider call: %v", err)
	}
	if cred.Token != "new-access" {
		t.Fatalf("credential token = %q, want refreshed", cred.Token)
	}
	if gotForm.Get("refresh_token") != "old-refresh" || gotForm.Get("grant_type") != "refresh_token" {
		t.Fatalf("refresh form = %v", gotForm)
	}
	// The rotated bundle is persisted, keeping the client secret.
	stored := readOAuthBundle(t, "work")
	if stored.AccessToken != "new-access" || stored.RefreshToken != "rotated-refresh" {
		t.Errorf("persisted bundle not rotated: %+v", stored)
	}
	if stored.ClientSecret != "secret" {
		t.Errorf("persisted bundle lost client secret: %+v", stored)
	}
}

func TestOAuthProviderRefreshFailureSurfacesReauth(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"expired"}`)
	}))
	defer srv.Close()
	defer swapOAuthEndpoints(oauth.Endpoints{Token: srv.URL + "/token"})()

	storeOAuthBundle(t, "work", oauth.TokenBundle{
		ClientSecret: "secret", AccessToken: "old", RefreshToken: "revoked",
		Expiry: time.Now().Add(-time.Minute),
	})
	provider, _ := oauthCredentialProvider("work", oauthProfile(), nil)
	_, err := provider(context.Background())
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeUnauthorized {
		t.Fatalf("error = %v, want unauthorized re-auth", err)
	}
}

func TestSiteClientWiresOAuthProvider(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := config.New()
	cfg.Sites["work"] = oauthProfile()
	if err := config.Save(configPath(t, dir), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	storeOAuthBundle(t, "work", oauth.TokenBundle{
		ClientSecret: "secret", AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour),
	})

	g := &GlobalFlags{Site: "work"}
	client, err := SiteClient(jiraInfo(), g)
	if err != nil {
		t.Fatalf("SiteClient: %v", err)
	}
	if client == nil {
		t.Fatal("SiteClient returned nil for oauth-3lo profile")
	}
}

func TestAcquireRefreshLockFailsFastWhenDirUnwritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permission bits enforced for a non-root user")
	}
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	start := time.Now()
	lockPath := filepath.Join(dir, "credentials.json.lock")
	release := acquireRefreshLock(lockPath)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("acquire took %v; an unwritable dir should proceed without the lock, not wait out the timeout", elapsed)
	}
	if release == nil {
		t.Fatal("release must never be nil")
	}
	release() // a no-op release must not panic
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("no lock file should exist in an unwritable dir: %v", err)
	}
}

func TestAcquireRefreshLockReclaimsStaleLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "credentials.json.lock")
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-2 * oauthRefreshLockTimeout)
	if err := os.Chtimes(lockPath, stale, stale); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	release := acquireRefreshLock(lockPath)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("acquire took %v; a stale lock should be reclaimed without waiting", elapsed)
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("reclaimed lock not present: %v", err)
	}
	if !info.ModTime().After(stale) {
		t.Fatal("lock file was not recreated after reclaiming the stale one")
	}
	release()
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file not removed on release: %v", err)
	}
}

func TestAcquireRefreshLockCreatesMissingDir(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "atlassian-cli", "credentials.json.lock")
	release := acquireRefreshLock(lockPath)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file not created: %v", err)
	}
	release()
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file not removed on release: %v", err)
	}
}

func TestAcquireRefreshLockDanglingSymlinkPollsUntilRemoved(t *testing.T) {
	// A dangling symlink at the lock path makes the exclusive create fail
	// (the name exists) while a following stat says it does not. The acquirer
	// must keep polling on its bounded cadence rather than spin, and return
	// once the link is gone.
	lockPath := filepath.Join(t.TempDir(), "credentials.json.lock")
	if err := os.Symlink(filepath.Join(filepath.Dir(lockPath), "missing"), lockPath); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		acquireRefreshLock(lockPath)()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("acquire returned while the dangling lock was still present")
	case <-time.After(150 * time.Millisecond):
	}
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("acquire did not return after the dangling lock was removed")
	}
}

func TestAcquireRefreshLockWaitsForHeldLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "credentials.json.lock")
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		acquireRefreshLock(lockPath)()
		close(done)
	}()
	time.Sleep(100 * time.Millisecond) // let the acquirer see the held lock and start polling
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("acquire did not return after the held lock was released")
	}
}
