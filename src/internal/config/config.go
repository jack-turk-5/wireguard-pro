// Package config loads application configuration from environment
// variables. Field/env var names match the original Python app's
// pydantic-settings config exactly, aside from dropping two fields
// (wg_public_key, ts) that were derived at runtime rather than configured --
// callers compute/hold those themselves.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration, loaded from environment
// variables by Load. The dashboard's JWT signing key is deliberately not
// part of this -- it's generated once and persisted by the application
// itself (see cmd/wireguard-pro/main.go's loadOrCreateSecretKey), the same
// way the WireGuard server key pair is, rather than living in the
// environment.
type Config struct {
	WGHost string
	WGPort string

	WGAllowedIPs   string
	WGDNSServer    string
	WGIPv4BaseAddr string
	WGIPv6BaseAddr string
	// WGMTU is the wg0 interface's MTU -- see docs/design-doc.md §2.5 on why
	// it defaults to 1420 rather than pasta's tap MTU: forwarded GSO skbs
	// are re-segmented at gso_size regardless of tap MTU.
	WGMTU       int
	DBFile      string
	NFTConfFile string
	LogLevel    string
}

// Load reads and validates configuration from the environment.
func Load() (*Config, error) {
	cfg := &Config{
		WGHost:         os.Getenv("WG_HOST"),
		WGPort:         os.Getenv("WG_PORT"),
		WGAllowedIPs:   envOr("WG_ALLOWED_IPS", "0.0.0.0/0, ::/0"),
		WGDNSServer:    envOr("WG_DNS_SERVER", "1.1.1.1"),
		WGIPv4BaseAddr: envOr("WG_IPV4_BASE_ADDR", "10.8.0.1"),
		WGIPv6BaseAddr: envOr("WG_IPV6_BASE_ADDR", "fd86:ea04:1111::1"),
		DBFile:         envOr("DB_FILE", "/data/peers.db"),
		NFTConfFile:    envOr("NFT_CONF_FILE", "/etc/nftables.json"),
		LogLevel:       envOr("LOG_LEVEL", "error"),
	}

	var missing []string
	if cfg.WGHost == "" {
		missing = append(missing, "WG_HOST")
	}
	if cfg.WGPort == "" {
		missing = append(missing, "WG_PORT")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("config: missing required environment variable(s): %v", missing)
	}

	mtu, err := envOrIntRange("WG_MTU", 1420, 1280, 65535)
	if err != nil {
		return nil, err
	}
	cfg.WGMTU = mtu

	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envOrIntRange parses key as an int, defaulting to def if unset, and
// validates it falls within [min, max].
func envOrIntRange(key string, def, min, max int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q is not an integer", key, v)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("config: %s=%d is outside the valid range [%d, %d]", key, n, min, max)
	}
	return n, nil
}
