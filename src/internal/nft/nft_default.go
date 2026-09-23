//go:build !linux

package nft

import (
	"fmt"
	"runtime"
)

// Apply is a stub on non-Linux platforms: nftables is a Linux kernel
// facility (nft.go/translate.go use netlink via google/nftables, which
// doesn't build here at all), so there's nothing to apply. Exists so
// cmd/wireguard-pro builds and `go vet`/`go test ./...` run cleanly on a
// macOS dev machine -- the real implementation only ever runs in the
// Linux container this project actually deploys as.
func Apply(path string) error {
	return fmt.Errorf("nft: not supported on %s (this is a Linux-only, container-deployed application)", runtime.GOOS)
}
