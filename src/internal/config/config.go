// Package config loads application configuration from environment
// variables. Field/env var names match the original Python app's
// pydantic-settings config exactly, aside from dropping two fields
// (wg_public_key, ts) that were derived at runtime rather than configured --
// callers compute/hold those themselves.
package config

import (
	"fmt"
	"os"
)

// Config holds all application configuration, loaded from environment
// variables by Load.
type Config struct {
	SecretKey string
	WGHost    string
	WGPort    string

	WGAllowedIPs   string
	WGDNSServer    string
	WGIPv4BaseAddr string
	WGIPv6BaseAddr string
	DBFile         string
	NFTConfFile    string
}

// Load reads and validates configuration from the environment.
func Load() (*Config, error) {
	cfg := &Config{
		SecretKey:      os.Getenv("SECRET_KEY"),
		WGHost:         os.Getenv("WG_HOST"),
		WGPort:         os.Getenv("WG_PORT"),
		WGAllowedIPs:   envOr("WG_ALLOWED_IPS", "0.0.0.0/0, ::/0"),
		WGDNSServer:    envOr("WG_DNS_SERVER", "1.1.1.1"),
		WGIPv4BaseAddr: envOr("WG_IPV4_BASE_ADDR", "10.8.0.1"),
		WGIPv6BaseAddr: envOr("WG_IPV6_BASE_ADDR", "fd86:ea04:1111::1"),
		DBFile:         envOr("DB_FILE", "/data/peers.db"),
		NFTConfFile:    envOr("NFT_CONF_FILE", "/etc/nftables.json"),
	}

	var missing []string
	if cfg.SecretKey == "" {
		missing = append(missing, "SECRET_KEY")
	}
	if cfg.WGHost == "" {
		missing = append(missing, "WG_HOST")
	}
	if cfg.WGPort == "" {
		missing = append(missing, "WG_PORT")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("config: missing required environment variable(s): %v", missing)
	}

	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
