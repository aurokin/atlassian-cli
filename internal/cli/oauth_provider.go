package cli

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/config"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
	"github.com/aurokin/atlassian-cli/internal/oauth"
	"github.com/aurokin/atlassian-cli/internal/secrets"
)

// oauthRefreshSkew refreshes an access token slightly before it actually
// expires, so a token that is about to lapse mid-request is renewed first.
const oauthRefreshSkew = 60 * time.Second

// oauthRefreshLockTimeout bounds how long a refresh waits for the advisory lock
// before assuming the holder crashed and reclaiming it. It must exceed the
// oauth HTTP client's request timeout so a waiter never reclaims the lock from
// a live-but-slow refresh (which would defeat single-flight); it is only ever
// reached when the holder actually crashed mid-refresh.
const oauthRefreshLockTimeout = 45 * time.Second

// oauthRefreshTimeout caps the token-refresh call itself, independently of
// --timeout (which may be raised or disabled). It must stay below
// oauthRefreshLockTimeout so a live refresh can never hold the lock long
// enough to be mistaken for a crashed run's leftover.
const oauthRefreshTimeout = 30 * time.Second

// newOAuthClient builds the OAuth client with the package's test seams
// (endpoints, clock) and the request-bounding *http.Client.
func newOAuthClient(clientID, clientSecret string, hc *http.Client) *oauth.Client {
	return oauth.New(clientID, clientSecret, oauth.Options{Endpoints: oauthEndpoints, Now: oauthNow, HTTPClient: hc})
}

// oauthCredentialProvider builds the request-time credential provider for an
// oauth-3lo profile. On each request it loads the stored token bundle and
// returns its access token if still valid; otherwise it refreshes via
// internal/oauth, persists the rotated bundle, and returns the new access
// token. A best-effort advisory file lock serializes the
// read→refresh→write-back so two concurrent atl-* runs do not each rotate the
// refresh token and invalidate the other's.
func oauthCredentialProvider(site string, profile config.SiteProfile, hc *http.Client) (httpclient.CredentialProvider, error) {
	credPath, err := config.CredentialsPath()
	if err != nil {
		return nil, err
	}
	store, err := secrets.ForRef(profile.TokenRef, credPath)
	if err != nil {
		return nil, err
	}
	lockPath := credPath + ".lock"

	loadBundle := func() (oauth.TokenBundle, error) {
		value, err := store.Get(site)
		if err != nil {
			return oauth.TokenBundle{}, err
		}
		return oauth.ParseBundle(value)
	}
	credFor := func(b oauth.TokenBundle) auth.Credential {
		return auth.Credential{Style: auth.StyleOAuth3LO, Token: b.AccessToken, CloudID: profile.CloudID}
	}

	return func(ctx context.Context) (auth.Credential, error) {
		bundle, err := loadBundle()
		if err != nil {
			return auth.Credential{}, err
		}
		// Fast path: a still-valid access token needs no lock or refresh.
		if !bundle.Expired(oauthNow().Add(oauthRefreshSkew)) {
			return credFor(bundle), nil
		}

		release := acquireRefreshLock(lockPath)
		defer release()

		// Re-read under the lock: another run may have refreshed while we waited.
		bundle, err = loadBundle()
		if err != nil {
			return auth.Credential{}, err
		}
		if !bundle.Expired(oauthNow().Add(oauthRefreshSkew)) {
			return credFor(bundle), nil
		}

		refreshCtx, cancel := context.WithTimeout(ctx, oauthRefreshTimeout)
		defer cancel()
		refreshed, err := newOAuthClient(profile.ClientID, bundle.ClientSecret, hc).Refresh(refreshCtx, bundle.RefreshToken)
		if err != nil {
			// internal/oauth already maps invalid_grant/invalid_token to a
			// re-authenticate apperr.
			return auth.Credential{}, err
		}
		// The token endpoint never echoes the client secret; carry it forward.
		// Atlassian rotates the refresh token, but keep the old one if a
		// response ever omits it so the grant is not lost.
		refreshed.ClientSecret = bundle.ClientSecret
		if refreshed.RefreshToken == "" {
			refreshed.RefreshToken = bundle.RefreshToken
		}
		value, err := refreshed.Marshal()
		if err != nil {
			return auth.Credential{}, err
		}
		if err := store.Set(site, value); err != nil {
			return auth.Credential{}, apperr.New("credential_write_failed",
				"could not persist the refreshed OAuth token: "+err.Error())
		}
		return credFor(refreshed), nil
	}, nil
}

// acquireRefreshLock takes a best-effort advisory lock by exclusively creating
// lockPath and returns a release func that removes it. The parent directory
// is created if missing (a keyring-backed profile never writes config.json).
// Only an already-existing lock file means "held" and is waited on; a lock
// file older than oauthRefreshLockTimeout is a crashed run's leftover (no
// live refresh holds it that long, see oauthRefreshTimeout) and is reclaimed
// once. Any other failure — an unwritable config dir, say — or a wait that
// outlasts the timeout returns a no-op release so the refresh proceeds
// without the lock: the lock only de-duplicates concurrent refreshes, so it
// is never worth stalling or failing a request over.
func acquireRefreshLock(lockPath string) func() {
	noop := func() {}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return noop
	}
	deadline := time.Now().Add(oauthRefreshLockTimeout)
	var staleAt time.Time // zero until the held lock has been stat'd
	reclaimed := false
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }
		}
		if !errors.Is(err, fs.ErrExist) {
			return noop
		}
		if !reclaimed && time.Now().After(staleAt) {
			// Stat on the first held sighting and again once the recorded
			// staleness instant passes, so a lock re-taken by a new holder in
			// the meantime is not removed. Two waiters can still race in the
			// instant between this check and the remove; the lock is
			// best-effort and that window is the price of not locking the lock.
			// Lstat, not Stat: a dangling symlink still makes O_EXCL fail, so
			// it must be seen (and age out) like any other leftover. A stat
			// failure falls through to the bounded poll rather than retrying
			// immediately, so nothing here can spin.
			if info, err := os.Lstat(lockPath); err == nil {
				staleAt = info.ModTime().Add(oauthRefreshLockTimeout)
				if time.Now().After(staleAt) {
					_ = os.Remove(lockPath)
					reclaimed = true
					continue
				}
			}
		}
		if time.Now().After(deadline) {
			return noop
		}
		time.Sleep(25 * time.Millisecond)
	}
}
