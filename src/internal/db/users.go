package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// AddUser inserts a new user with a bcrypt-hashed password. Reports false
// (no error) if the username already exists, matching the original app's
// IntegrityError-swallowing behavior.
func (d *DB) AddUser(username, password string) (bool, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, fmt.Errorf("db: hash password for %s: %w", username, err)
	}

	_, err = d.sql.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, hash)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return false, nil
		}
		return false, fmt.Errorf("db: add user %s: %w", username, err)
	}
	return true, nil
}

// UpsertUser creates the user or updates their password hash if they already
// exist -- used for seeding/rotating the admin account at startup.
func (d *DB) UpsertUser(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("db: hash password for %s: %w", username, err)
	}

	_, err = d.sql.Exec(
		`INSERT INTO users (username, password_hash) VALUES (?, ?)
		 ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash`,
		username, hash,
	)
	if err != nil {
		return fmt.Errorf("db: upsert user %s: %w", username, err)
	}
	return nil
}

// VerifyUser reports whether password matches the stored hash for username.
// A nonexistent user is treated as a failed verification, not an error.
func (d *DB) VerifyUser(username, password string) (bool, error) {
	var hash []byte
	err := d.sql.QueryRow(`SELECT password_hash FROM users WHERE username = ?`, username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("db: verify user %s: %w", username, err)
	}

	return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil, nil
}

// RemoveUser deletes a user by username. Reports whether a row was deleted.
func (d *DB) RemoveUser(username string) (bool, error) {
	res, err := d.sql.Exec(`DELETE FROM users WHERE username = ?`, username)
	if err != nil {
		return false, fmt.Errorf("db: remove user %s: %w", username, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("db: remove user %s: %w", username, err)
	}
	return n > 0, nil
}

// isUniqueConstraintErr reports whether err is a UNIQUE constraint violation.
// modernc.org/sqlite wraps the sqlite3 error message rather than exposing a
// typed error for this, so string matching is the documented way to detect it.
func isUniqueConstraintErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
