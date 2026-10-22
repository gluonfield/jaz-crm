// Package storage holds the durable data contracts. Records mirror the sqlc
// models field for field so the Postgres adapter converts rows with a plain
// type conversion; a schema change fails to compile until the record follows.
package storage

import "errors"

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
	// ErrReused reports a rotated refresh token presented again; its grant is revoked.
	ErrReused = errors.New("token reused")
	// ErrUnexpected wraps storage failures callers should not show to users.
	ErrUnexpected = errors.New("unexpected storage error")
)
