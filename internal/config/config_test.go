package config_test

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"patchboard/internal/config"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad_RequiresBaseURL(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(env(map[string]string{"PB_DATA_DIR": dir})); err == nil {
		t.Fatal("expected error without PB_BASE_URL")
	}
	if _, err := config.Load(env(map[string]string{"PB_DATA_DIR": dir, "PB_BASE_URL": "ftp://x"})); err == nil {
		t.Fatal("expected error for ftp base URL")
	}
}

func TestLoad_SecretPersistsInDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	e := env(map[string]string{"PB_DATA_DIR": dir, "PB_BASE_URL": "http://localhost:8080"})
	a, err := config.Load(e)
	if err != nil {
		t.Fatal(err)
	}
	b, err := config.Load(e)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Secret) != 32 || string(a.Secret) != string(b.Secret) {
		t.Fatalf("secrets differ or wrong length: %d", len(a.Secret))
	}
	st, err := os.Stat(filepath.Join(dir, "secret"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("secret mode = %v", st.Mode().Perm())
	}
}

func TestLoad_TrustedProxies(t *testing.T) {
	dir := t.TempDir()
	c, err := config.Load(env(map[string]string{"PB_DATA_DIR": dir, "PB_BASE_URL": "http://x", "PB_TRUSTED_PROXIES": "10.0.0.0/8, 172.16.0.1"}))
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.1/32")}
	if len(c.TrustedProxies) != 2 || c.TrustedProxies[0] != want[0] || c.TrustedProxies[1] != want[1] {
		t.Fatalf("proxies = %v", c.TrustedProxies)
	}
	if _, err := config.Load(env(map[string]string{"PB_DATA_DIR": dir, "PB_BASE_URL": "http://x", "PB_TRUSTED_PROXIES": "nope"})); err == nil {
		t.Fatal("expected error for invalid proxy")
	}
}

func TestLoad_SecureCookiesFromScheme(t *testing.T) {
	dir := t.TempDir()
	c, _ := config.Load(env(map[string]string{"PB_DATA_DIR": dir, "PB_BASE_URL": "https://feedback.example.com/"}))
	if !c.SecureCookies {
		t.Fatal("https should enable secure cookies")
	}
	if c.BaseURL.String() != "https://feedback.example.com" {
		t.Fatalf("base url = %s", c.BaseURL)
	}
	c, _ = config.Load(env(map[string]string{"PB_DATA_DIR": dir, "PB_BASE_URL": "http://localhost:8080"}))
	if c.SecureCookies {
		t.Fatal("http should not enable secure cookies")
	}
}
