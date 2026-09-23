package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// Peer mirrors the peers table. created_at/expires_at are kept as the
// "2006-01-02 15:04:05" UTC strings the original app used, rather than
// switching to a different format, since nothing downstream needs anything
// richer than string comparison/display.
type Peer struct {
	PublicKey   string
	PrivateKey  string
	Nickname    string
	IPv4Address string
	IPv6Address string
	CreatedAt   string
	ExpiresAt   string
}

// AddPeer inserts a new peer row.
func (d *DB) AddPeer(publicKey, privateKey, ipv4, ipv6, expiresAt string) error {
	_, err := d.sql.Exec(
		`INSERT INTO peers (public_key, private_key, ipv4_address, ipv6_address, expires_at)
		 VALUES (?, ?, ?, ?, ?)`,
		publicKey, privateKey, ipv4, ipv6, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("db: add peer %s: %w", publicKey, err)
	}
	return nil
}

// RemovePeer deletes a peer by public key. Reports whether a row was deleted.
func (d *DB) RemovePeer(publicKey string) (bool, error) {
	res, err := d.sql.Exec(`DELETE FROM peers WHERE public_key = ?`, publicKey)
	if err != nil {
		return false, fmt.Errorf("db: remove peer %s: %w", publicKey, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("db: remove peer %s: %w", publicKey, err)
	}
	return n > 0, nil
}

// RenamePeer sets a peer's nickname. Reports whether a row was updated.
func (d *DB) RenamePeer(publicKey, nickname string) (bool, error) {
	res, err := d.sql.Exec(`UPDATE peers SET nickname = ? WHERE public_key = ?`, nickname, publicKey)
	if err != nil {
		return false, fmt.Errorf("db: rename peer %s: %w", publicKey, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("db: rename peer %s: %w", publicKey, err)
	}
	return n > 0, nil
}

// ListPeers returns all stored peers.
func (d *DB) ListPeers() ([]Peer, error) {
	rows, err := d.sql.Query(
		`SELECT public_key, private_key, nickname, ipv4_address, ipv6_address, created_at, expires_at FROM peers`,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list peers: %w", err)
	}
	defer rows.Close()

	var peers []Peer
	for rows.Next() {
		var p Peer
		if err := rows.Scan(&p.PublicKey, &p.PrivateKey, &p.Nickname, &p.IPv4Address, &p.IPv6Address, &p.CreatedAt, &p.ExpiresAt); err != nil {
			return nil, fmt.Errorf("db: list peers: scan: %w", err)
		}
		peers = append(peers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list peers: %w", err)
	}
	return peers, nil
}

// GetPeer looks up a single peer by public key.
func (d *DB) GetPeer(publicKey string) (Peer, error) {
	var p Peer
	err := d.sql.QueryRow(
		`SELECT public_key, private_key, nickname, ipv4_address, ipv6_address, created_at, expires_at
		 FROM peers WHERE public_key = ?`,
		publicKey,
	).Scan(&p.PublicKey, &p.PrivateKey, &p.Nickname, &p.IPv4Address, &p.IPv6Address, &p.CreatedAt, &p.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Peer{}, fmt.Errorf("db: get peer %s: %w", publicKey, sql.ErrNoRows)
	}
	if err != nil {
		return Peer{}, fmt.Errorf("db: get peer %s: %w", publicKey, err)
	}
	return p, nil
}
