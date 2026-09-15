package e2e

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/secrets"
)

// TestNativeCredentialLifecycle is intended for disposable native CI workers.
// Unlike ordinary contracts, this explicit opt-in writes dummy credentials to
// the OS keychain. Every key is random, checked absent first, and cleaned up.
func TestNativeCredentialLifecycle(t *testing.T) {
	if os.Getenv("ATL_E2E_NATIVE_CREDENTIALS") != "1" {
		t.Skip("set ATL_E2E_NATIVE_CREDENTIALS=1 only on an ephemeral native test worker")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("native credential acceptance targets macOS Keychain and Windows Credential Manager")
	}
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			var random [16]byte
			if _, err := rand.Read(random[:]); err != nil {
				t.Fatal(err)
			}
			prefix := "atl-native-e2e-" + hex.EncodeToString(random[:])
			primary, sibling := prefix+"-primary", prefix+"-sibling"
			store, err := secrets.ForRef(secrets.BackendKeyring, filepath.Join(p.home, "atlassian-cli", "credentials.json"))
			if err != nil {
				t.Fatal("could not open native credential provider")
			}
			for _, site := range []string{primary, sibling} {
				present, err := store.Has(site)
				if err != nil {
					t.Fatal("native credential presence check failed")
				}
				if present {
					t.Fatal("random native credential key already exists; refusing to overwrite")
				}
				t.Cleanup(func() {
					if err := store.Delete(site); err != nil {
						t.Error("cleanup failed for owned native credential")
					}
					if present, err := store.Has(site); err != nil || present {
						t.Error("owned native credential remains after cleanup")
					}
				})
			}
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				wantPath := map[string]string{"jira": "/myself", "conf": "/rest/api/user/current", "bb": "/user"}[product]
				if r.URL.Path != wantPath {
					t.Errorf("wrong status endpoint %s", r.URL.Path)
				}
				var account string
				switch r.Header.Get("Authorization") {
				case "Bearer " + dummyToken:
					account = "primary-account"
				case "Bearer " + dummyToken + "-sibling":
					account = "sibling-account"
				default:
					t.Error("status request did not use the expected stored credential")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				field := "accountId"
				if product == "bb" {
					field = "account_id"
				}
				_, _ = fmt.Fprintf(w, `{%q:%q}`, field, account)
			}))
			defer srv.Close()
			for _, entry := range []struct{ site, token string }{{primary, dummyToken}, {sibling, dummyToken + "-sibling"}} {
				r := sharedStdin(t, p, product, entry.token+"\n", false,
					"auth", "login", "--site", entry.site, "--url", srv.URL,
					"--token-style", "data-center-pat", "--token-stdin", "--no-prompt")
				success(t, r)
				if sharedConfig(t, p).Sites[entry.site].TokenRef != secrets.BackendKeyring {
					t.Fatal("native login fell back instead of storing in the OS keychain")
				}
				if present, err := store.Has(entry.site); err != nil || !present {
					t.Fatal("native login did not persist its credential")
				}
			}
			if _, err := os.Stat(filepath.Join(p.home, "atlassian-cli", "credentials.json")); !os.IsNotExist(err) {
				t.Fatal("native credential acceptance unexpectedly created a fallback credential file")
			}
			// Credentials must survive between processes without a token env var.
			p.env = append(p.env, "ATL_E2E_TOKEN=")
			checkStatus := func(site, account string) {
				t.Helper()
				field := ".accountId"
				if product == "bb" {
					field = ".account_id"
				}
				r := p.run(t, product, "status", "--site", site, "--jq", field)
				success(t, r)
				if strings.TrimSpace(r.stdout) != fmt.Sprintf("%q", account) {
					t.Fatal("status authenticated with the wrong native credential")
				}
			}
			checkStatus(primary, "primary-account")
			checkStatus(sibling, "sibling-account")
			r := p.run(t, product, "auth", "status", "--site", primary, "--jq", ".token_status")
			success(t, r)
			if !strings.Contains(r.stdout, "OS keychain") {
				t.Fatal("auth status did not report native credential availability")
			}
			success(t, p.run(t, product, "auth", "logout", "--site", primary))
			if present, err := store.Has(primary); err != nil || present {
				t.Fatal("logout retained primary native credential")
			}
			if present, err := store.Has(sibling); err != nil || !present {
				t.Fatal("logout erased sibling native credential")
			}
			checkStatus(sibling, "sibling-account")
			if requests.Load() != 3 {
				t.Fatal("native credential status requests missing or unexpected")
			}
			success(t, p.run(t, product, "auth", "logout", "--site", sibling))
			if present, err := store.Has(sibling); err != nil || present {
				t.Fatal("logout retained sibling native credential")
			}
			if len(sharedConfig(t, p).Sites) != 0 {
				t.Fatal("native logout retained profiles")
			}
		})
	}
}
