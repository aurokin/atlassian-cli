package appinfo

import "testing"

func TestNewDefaultsEmptyVersionToDev(t *testing.T) {
	got := New("atl-jira", ProductJira, "", "", "")
	if got.Version != "dev" {
		t.Fatalf("Version = %q, want %q", got.Version, "dev")
	}
}

func TestNewPreservesBinaryAndProduct(t *testing.T) {
	got := New("atl-conf", ProductConfluence, "1.2.3", "abc123", "2026-05-17")
	if got.Binary != "atl-conf" {
		t.Errorf("Binary = %q, want %q", got.Binary, "atl-conf")
	}
	if got.Product != ProductConfluence {
		t.Errorf("Product = %q, want %q", got.Product, ProductConfluence)
	}
	if got.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", got.Version, "1.2.3")
	}
	if got.Commit != "abc123" {
		t.Errorf("Commit = %q, want %q", got.Commit, "abc123")
	}
	if got.Date != "2026-05-17" {
		t.Errorf("Date = %q, want %q", got.Date, "2026-05-17")
	}
}
