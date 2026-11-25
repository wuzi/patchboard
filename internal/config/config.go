// Package config loads PatchBoard's settings from environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr           string
	DataDir        string
	BaseURL        *url.URL
	TrustedProxies []netip.Prefix
	Secret         []byte
	SecureCookies  bool
}

// Load reads PB_* variables through getenv.
func Load(getenv func(string) string) (Config, error) {
	var c Config
	c.Addr = getenv("PB_ADDR")
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	c.DataDir = getenv("PB_DATA_DIR")
	if c.DataDir == "" {
		c.DataDir = "./data"
	}
	if err := os.MkdirAll(c.DataDir, 0o700); err != nil {
		return c, fmt.Errorf("create data dir: %w", err)
	}

	raw := strings.TrimRight(strings.TrimSpace(getenv("PB_BASE_URL")), "/")
	if raw == "" {
		return c, errors.New("PB_BASE_URL is required (e.g. https://feedback.example.com)")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return c, fmt.Errorf("PB_BASE_URL must be an absolute http(s) URL, got %q", raw)
	}
	c.BaseURL = u
	c.SecureCookies = u.Scheme == "https"

	proxies, err := parseProxies(getenv("PB_TRUSTED_PROXIES"))
	if err != nil {
		return c, err
	}
	c.TrustedProxies = proxies

	if s := getenv("PB_SECRET"); s != "" {
		if len(s) < 32 {
			return c, errors.New("PB_SECRET must be at least 32 characters")
		}
		c.Secret = []byte(s)
	} else {
		sec, err := loadOrCreateSecret(filepath.Join(c.DataDir, "secret"))
		if err != nil {
			return c, err
		}
		c.Secret = sec
	}
	return c, nil
}

func parseProxies(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("PB_TRUSTED_PROXIES: %w", err)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("PB_TRUSTED_PROXIES: %w", err)
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

func loadOrCreateSecret(path string) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		sec, err := hex.DecodeString(strings.TrimSpace(string(b)))
		if err != nil || len(sec) < 32 {
			return nil, fmt.Errorf("invalid secret file %s", path)
		}
		return sec, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	sec := make([]byte, 32)
	if _, err := rand.Read(sec); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(sec)), 0o600); err != nil {
		return nil, fmt.Errorf("write secret: %w", err)
	}
	return sec, nil
}
