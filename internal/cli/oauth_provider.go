package cli

import (
	"context"
	"os"
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

// oauthCredentialProvider builds the request-time credential provider for an
// oauth-3lo profile. On each request it loads the stored token bundle and
// returns its access token if still valid; otherwise it refreshes via
// internal/oauth, persists the rotated bundle, and returns the new access
// token. A best-effort advisory file lock serializes the
// read→refresh→write-back so two concurrent atl-* runs do not each rotate the
// refresh token and invalidate the other's.
func oauthCredentialProvider(site string, profile config.SiteProfile) (httpclient.CredentialProvider, error) {
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

		client := oauth.New(profile.ClientID, bundle.ClientSecret, oauth.Options{Endpoints: oauthEndpoints, Now: oauthNow})
		refreshed, err := client.Refresh(ctx, bundle.RefreshToken)
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
// lockPath, returning a release func that removes it. If the lock is held past
// oauthRefreshLockTimeout it is assumed stale (a crashed run) and reclaimed;
// if it still cannot be taken, the caller proceeds without it rather than
// failing a refresh.
func acquireRefreshLock(lockPath string) func() {
	deadline := time.Now().Add(oauthRefreshLockTimeout)
	for {
		if f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }
		}
		if !time.Now().Before(deadline) {
			// Assume a stale lock from a crashed run and reclaim it once.
			_ = os.Remove(lockPath)
			if f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err == nil {
				_ = f.Close()
				return func() { _ = os.Remove(lockPath) }
			}
			return func() {}
		}
		time.Sleep(25 * time.Millisecond)
	}
}
