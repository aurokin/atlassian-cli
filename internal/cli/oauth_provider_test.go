package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	provider, err := oauthCredentialProvider("work", oauthProfile())
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
	provider, err := oauthCredentialProvider("work", oauthProfile())
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
	provider, _ := oauthCredentialProvider("work", oauthProfile())
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
