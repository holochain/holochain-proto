// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Hash type for Holochains
// Holochain hashes are SHA256 binary values encoded to strings as base58

package holochain

import (
	"bytes"
	"encoding/binary"
	"errors"
	mh "github.com/multiformats/go-multihash"
	"io"
)

// Hash of Entry's Content
type Hash struct {
	H mh.Multihash
}

// HashSpec holds the info that tells what kind of hash this is
type HashSpec struct {
	Code   uint64
	Length int
}

// NewHash builds a Hash from a b58 string encoded hash
func NewHash(s string) (h Hash, err error) {
	h.H, err = mh.FromB58String(s)
	return
}

// String encodes a hash to a human readable string
func (h Hash) String() string {
	if cap(h.H) == 0 {
		return ""
	}
	return h.H.B58String()
}

// Sum builds a digest according to the specs in the Holochain
func (h *Hash) Sum(hc HashSpec, data []byte) (err error) {
	h.H, err = mh.Sum(data, hc.Code, hc.Length)
	return
}

// IsNullHash checks to see if this hash's value is the null hash
func (h *Hash) IsNullHash() bool {
	return cap(h.H) == 1 && h.H[0] == 0
}

// NullHash builds a null valued hash
func NullHash() (h Hash) {
	null := [1]byte{0}
	h.H = null[:]
	return
}

// Clone returns a copy of a hash
func (h *Hash) Clone() (hash Hash) {
	hash.H = make([]byte, len(h.H))
	copy(hash.H, h.H)
	return
}

// Equal checks to see if two hashes have the same value
func (h *Hash) Equal(h2 *Hash) bool {
	if h.IsNullHash() && h2.IsNullHash() {
		return true
	}
	return bytes.Equal(h.H, h2.H)
}

// MarshalHash writes a hash to a binary stream
func (h *Hash) MarshalHash(writer io.Writer) (err error) {
	if h.IsNullHash() {
		b := make([]byte, 34)
		err = binary.Write(writer, binary.LittleEndian, b)
	} else {
		if h.H == nil {
			err = errors.New("can't marshal nil hash")
		} else {
			err = binary.Write(writer, binary.LittleEndian, h.H)
		}
	}
	return
}

// UnmarshalHash reads a hash from a binary stream
func (h *Hash) UnmarshalHash(reader io.Reader) (err error) {
	b := make([]byte, 34)
	err = binary.Read(reader, binary.LittleEndian, b)
	if err == nil {
		if b[0] == 0 {
			h.H = NullHash().H
		} else {
			h.H = b
		}
	}
	return
}
