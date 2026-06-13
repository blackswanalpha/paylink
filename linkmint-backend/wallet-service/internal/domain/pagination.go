package domain

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// DefaultPageLimit and MaxPageLimit bound list endpoints.
const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

// ClampLimit returns a sane page size: def when limit<=0, capped at MaxPageLimit.
func ClampLimit(limit int) int {
	if limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return limit
}

// EncodeCursor builds an opaque keyset cursor over (block_height, id) — the ordering both list
// endpoints use (block_height DESC, id DESC).
func EncodeCursor(blockHeight uint64, id string) string {
	raw := fmt.Sprintf("%020d|%s", blockHeight, id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a cursor produced by EncodeCursor. ok is false for an empty or malformed
// cursor (callers then start from the newest row).
func DecodeCursor(cursor string) (blockHeight uint64, id string, ok bool) {
	if cursor == "" {
		return 0, "", false
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", false
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	bh, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return bh, parts[1], true
}
