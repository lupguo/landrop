package store

import (
	"crypto/rand"
	"time"
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newID returns a 26-char ULID-like ID: 48-bit ms timestamp + 80 random bits.
func newID(t time.Time) (string, error) {
	var b [16]byte
	ms := uint64(t.UnixMilli())
	b[0], b[1], b[2], b[3], b[4], b[5] = byte(ms>>40), byte(ms>>32), byte(ms>>24), byte(ms>>16), byte(ms>>8), byte(ms)
	if _, err := rand.Read(b[6:]); err != nil {
		return "", err
	}
	// 128 bits -> 26 groups of 5 bits (top group has 3 bits).
	out := make([]byte, 26)
	var acc, bits uint
	pos := 25
	for i := 15; i >= 0; i-- {
		acc |= uint(b[i]) << bits
		bits += 8
		for bits >= 5 {
			out[pos] = crockford[acc&31]
			pos--
			acc >>= 5
			bits -= 5
		}
	}
	if bits > 0 {
		out[pos] = crockford[acc&31]
	}
	return string(out), nil
}
