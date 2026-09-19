//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/config"
	"github.com/aurokin/atlassian-cli/internal/oauth"
	"github.com/aurokin/atlassian-cli/internal/secrets"
)

// TestOAuthRefreshPersistence is an explicitly selected live grant mutation:
// ATL_IT_OAUTH_SITE names a dedicated test profile whose local expiry may be
// changed. Run serially after browser reauthorization, in stored-profile mode:
//
//	ATL_RUN_INTEGRATION=1 ATL_IT_USE_STORED_PROFILES=1 ATL_IT_OAUTH_SITE=work \
//	  go test -tags=integration ./integration -run '^TestOAuthRefreshPersistence$' -count=1 -v
//
// No grant is revoked. Successful refresh remains persisted; restoring an old
// bundle would overwrite the newly rotated refresh token and invalidate it.
func TestOAuthRefreshPersistence(t *testing.T) {
	requireIntegration(t)
	site := strings.TrimSpace(os.Getenv("ATL_IT_OAUTH_SITE"))
	if site == "" {
		t.Skip("set ATL_IT_OAUTH_SITE to explicitly authorize expiry mutation of one dedicated test OAuth profile")
	}
	if !useStoredProfiles() {
		t.Fatal("OAuth refresh acceptance requires ATL_IT_USE_STORED_PROFILES=1")
	}
	path, err := config.DefaultPath()
	if err != nil {
		t.Fatal("resolve OAuth profile config path")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load OAuth profile config")
	}
	profile, ok := cfg.Sites[site]
	if !ok || profile.TokenStyle != "oauth-3lo" {
		t.Fatal("selected OAuth profile is absent or is not oauth-3lo")
	}
	var binary string
	switch profile.Product {
	case "jira":
		binary = "atl-jira"
	case "confluence":
		binary = "atl-conf"
	default:
		t.Fatal("OAuth refresh acceptance supports only Jira or Confluence")
	}
	if profile.ClientID == "" || profile.CloudID == "" {
		t.Fatal("selected OAuth profile lacks client ID or cloud ID")
	}
	credPath, err := config.CredentialsPath()
	if err != nil {
		t.Fatal("resolve OAuth credential backend")
	}
	store, err := secrets.ForRef(profile.TokenRef, credPath)
	if err != nil {
		t.Fatal("open OAuth credential backend")
	}
	load := func() oauth.TokenBundle {
		t.Helper()
		value, err := store.Get(site)
		if err != nil {
			t.Fatal("load dedicated OAuth bundle")
		}
		bundle, err := oauth.ParseBundle(value)
		if err != nil {
			t.Fatal("parse dedicated OAuth bundle")
		}
		if bundle.AccessToken == "" || bundle.RefreshToken == "" || bundle.ClientSecret == "" {
			t.Fatal("dedicated OAuth bundle is incomplete; reauthorize before testing refresh")
		}
		return bundle
	}
	s := &session{t: t, binaryPath: buildBinary(t, binary), site: site}
	// Do not print raw status diagnostics: even a regression leaking a credential
	// must produce only a redacted test failure.
	status := func(bundle oauth.TokenBundle) (string, cmdResult) {
		t.Helper()
		result := s.run("status", "--json")
		for _, secret := range []string{bundle.AccessToken, bundle.RefreshToken, bundle.ClientSecret} {
			if strings.Contains(result.stdout+result.stderr, secret) {
				t.Fatal("status output exposed an OAuth credential (redacted)")
			}
		}
		if result.err != nil {
			t.Fatal("OAuth live status failed; diagnostics withheld to protect credentials")
		}
		var identity struct {
			AccountID string `json:"accountId"`
		}
		if err := json.Unmarshal([]byte(result.stdout), &identity); err != nil || identity.AccountID == "" {
			t.Fatal("OAuth status did not return an authenticated account ID")
		}
		return identity.AccountID, result
	}
	initial := load()
	account, _ := status(initial)
	// The preflight itself may have refreshed an already-expired token. Use its
	// persisted bundle as the baseline for the deliberately forced refresh.
	before := load()
	expired := before
	expired.Expiry = time.Now().Add(-time.Minute)
	value, err := expired.Marshal()
	if err != nil {
		t.Fatal("serialize expired OAuth test bundle")
	}
	t.Logf("forcing local OAuth expiry for explicitly selected profile: product=%s site=%s", profile.Product, site)
	if err := store.Set(site, value); err != nil {
		t.Fatal("persist dedicated OAuth expiry mutation")
	}
	started := time.Now()
	refreshedAccount, result := status(before)
	after := load()
	if refreshedAccount != account {
		t.Fatal("OAuth refresh changed authenticated identity")
	}
	if after.AccessToken == before.AccessToken {
		t.Fatal("forced OAuth refresh did not change the persisted access token")
	}
	if after.RefreshToken == before.RefreshToken {
		t.Fatal("forced OAuth refresh did not persist a rotated refresh token")
	}
	if !after.Expiry.After(started.Add(time.Minute)) {
		t.Fatal("forced OAuth refresh did not persist a renewed expiry")
	}
	if after.ClientSecret != before.ClientSecret {
		t.Fatal("OAuth refresh failed to preserve the client secret")
	}
	for _, secret := range []string{after.AccessToken, after.RefreshToken, after.ClientSecret} {
		if strings.Contains(result.stdout+result.stderr, secret) {
			t.Fatal("refresh output exposed a newly issued OAuth credential (redacted)")
		}
	}
	secondAccount, _ := status(after)
	persisted := load()
	if secondAccount != account {
		t.Fatal("second OAuth process changed authenticated identity")
	}
	if persisted.AccessToken != after.AccessToken || persisted.RefreshToken != after.RefreshToken || !persisted.Expiry.Equal(after.Expiry) {
		t.Fatal("second OAuth process did not reuse the persisted refreshed bundle")
	}
	t.Logf("OAuth acceptance: product=%s site=%s forced_refresh=true rotated_refresh_persisted=true renewed_expiry=true second_process=true", profile.Product, site)
}
