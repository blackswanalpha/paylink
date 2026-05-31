package types

import (
	"encoding/hex"
	"fmt"
)

const (
	HashLength    = 32
	AddressLength = 20
)

// Hash represents a 32-byte hash (SHA-256 or Keccak-256).
type Hash [HashLength]byte

// Address represents a 20-byte account address.
type Address [AddressLength]byte

var (
	ZeroHash    Hash
	ZeroAddress Address
)

// BytesToHash converts a byte slice to a Hash, left-padding or truncating as needed.
func BytesToHash(b []byte) Hash {
	var h Hash
	if len(b) > HashLength {
		b = b[len(b)-HashLength:]
	}
	copy(h[HashLength-len(b):], b)
	return h
}

// HexToHash converts a hex string (with or without 0x prefix) to a Hash.
func HexToHash(s string) Hash {
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	b, _ := hex.DecodeString(s)
	return BytesToHash(b)
}

// Bytes returns the hash as a byte slice.
func (h Hash) Bytes() []byte {
	return h[:]
}

// Hex returns the hex string representation with 0x prefix.
func (h Hash) Hex() string {
	return "0x" + hex.EncodeToString(h[:])
}

// String implements fmt.Stringer.
func (h Hash) String() string {
	return h.Hex()
}

// IsZero returns true if the hash is all zeros.
func (h Hash) IsZero() bool {
	return h == ZeroHash
}

// MarshalJSON implements json.Marshaler.
func (h Hash) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, h.Hex())), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (h *Hash) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid hash JSON: %s", string(data))
	}
	s := string(data[1 : len(data)-1])
	*h = HexToHash(s)
	return nil
}

// BytesToAddress converts a byte slice to an Address.
func BytesToAddress(b []byte) Address {
	var a Address
	if len(b) > AddressLength {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
	return a
}

// HexToAddress converts a hex string to an Address.
func HexToAddress(s string) Address {
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	b, _ := hex.DecodeString(s)
	return BytesToAddress(b)
}

// Bytes returns the address as a byte slice.
func (a Address) Bytes() []byte {
	return a[:]
}

// Hex returns the hex string representation with 0x prefix.
func (a Address) Hex() string {
	return "0x" + hex.EncodeToString(a[:])
}

// String implements fmt.Stringer.
func (a Address) String() string {
	return a.Hex()
}

// IsZero returns true if the address is all zeros.
func (a Address) IsZero() bool {
	return a == ZeroAddress
}

// MarshalJSON implements json.Marshaler.
func (a Address) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, a.Hex())), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *Address) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid address JSON: %s", string(data))
	}
	s := string(data[1 : len(data)-1])
	*a = HexToAddress(s)
	return nil
}
