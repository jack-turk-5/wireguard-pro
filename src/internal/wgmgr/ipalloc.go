package wgmgr

import "fmt"

// NextAvailableIP allocates the next free IPv4/IPv6 address for a new peer,
// replicating the original app's scheme exactly: ipv4Base/ipv6Base are the
// *server's* addresses (e.g. "10.8.0.1", "fd86:ea04:1111::1") with their
// trailing character stripped to form a prefix, then peers are assigned
// sequential suffixes -- "10.8.0.2".."10.8.0.254", and
// "fd86:ea04:1111::100".."fd86:ea04:1111::ffff" in hex.
func NextAvailableIP(ipv4Base, ipv6Base string, usedV4, usedV6 map[string]bool) (string, string, error) {
	if len(ipv4Base) == 0 || len(ipv6Base) == 0 {
		return "", "", fmt.Errorf("wgmgr: empty base address")
	}

	v4Prefix := ipv4Base[:len(ipv4Base)-1]
	ipv4 := ""
	for i := 2; i < 255; i++ {
		candidate := fmt.Sprintf("%s%d", v4Prefix, i)
		if !usedV4[candidate] {
			ipv4 = candidate
			break
		}
	}
	if ipv4 == "" {
		return "", "", fmt.Errorf("wgmgr: no free IPv4 addresses left in %s0/24", v4Prefix)
	}

	v6Prefix := ipv6Base[:len(ipv6Base)-1]
	ipv6 := ""
	for suffix := 0x100; suffix < 0x10000; suffix++ {
		candidate := fmt.Sprintf("%s%x", v6Prefix, suffix)
		if !usedV6[candidate] {
			ipv6 = candidate
			break
		}
	}
	if ipv6 == "" {
		return "", "", fmt.Errorf("wgmgr: no free IPv6 addresses left in %s/64", v6Prefix)
	}

	return ipv4, ipv6, nil
}
