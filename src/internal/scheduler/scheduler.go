// Package scheduler runs the expired-peer sweep on a fixed interval,
// replacing the original Python app's apscheduler-driven hourly job.
package scheduler

import (
	"context"
	"log"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"wireguard-pro/internal/db"
	"wireguard-pro/internal/wgmgr"
)

// expiryTimeLayout matches the format peer expiry timestamps are stored in
// (see internal/db and the peer-creation handler in internal/api).
const expiryTimeLayout = "2006-01-02 15:04:05"

// RunExpirySweep removes expired peers immediately, then again every
// interval, until ctx is canceled. Intended to run in its own goroutine.
func RunExpirySweep(ctx context.Context, database *db.DB, wgm *wgmgr.Client, interval time.Duration) {
	sweepExpiredPeers(database, wgm)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweepExpiredPeers(database, wgm)
		}
	}
}

// sweepExpiredPeers removes every peer whose expires_at has passed, from
// both the DB and the running device.
func sweepExpiredPeers(database *db.DB, wgm *wgmgr.Client) {
	peers, err := database.ListPeers()
	if err != nil {
		log.Printf("scheduler: list peers: %v", err)
		return
	}

	now := time.Now().UTC()
	removed := 0
	for _, p := range peers {
		expires, err := time.Parse(expiryTimeLayout, p.ExpiresAt)
		if err != nil {
			log.Printf("scheduler: parse expires_at for peer %s: %v", p.PublicKey, err)
			continue
		}
		if expires.After(now) {
			continue
		}

		ok, err := database.RemovePeer(p.PublicKey)
		if err != nil {
			log.Printf("scheduler: remove peer %s from db: %v", p.PublicKey, err)
			continue
		}
		if !ok {
			continue
		}

		pub, err := wgtypes.ParseKey(p.PublicKey)
		if err != nil {
			log.Printf("scheduler: parse public key %s: %v", p.PublicKey, err)
			continue
		}
		if err := wgm.RemovePeer(pub); err != nil {
			log.Printf("scheduler: remove peer %s from device: %v", p.PublicKey, err)
			continue
		}
		removed++
	}

	if removed > 0 {
		log.Printf("scheduler: auto-expired and removed %d peer(s)", removed)
	} else {
		log.Printf("scheduler: no expired peers found")
	}
}
