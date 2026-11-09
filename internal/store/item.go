// Package store is the in-memory item index; text lives in memory, files are streamed to disk.
package store

import (
	"errors"
	"time"
)

type Kind string

const (
	KindText Kind = "text"
	KindFile Kind = "file"
)

// MaxText is the maximum text item size in bytes.
const MaxText = 1 << 20

var (
	ErrTooLarge = errors.New("store: too large")
	ErrEmpty    = errors.New("store: empty")
	ErrNotFound = errors.New("store: not found")
)

type FileMeta struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	MIME string `json:"mime"`
}

type From struct {
	PeerID string `json:"peer_id"`
	Name   string `json:"name"`
}

type Item struct {
	ID        string    `json:"id"`
	Seq       uint64    `json:"seq"`
	Type      Kind      `json:"type"`
	Content   string    `json:"content,omitempty"`
	File      *FileMeta `json:"file,omitempty"`
	From      From      `json:"from"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
