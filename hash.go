// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Hash type for Holochains
// Holochain hashes are SHA256 binary values encoded to strings as base58

package holochain

import (
	. "github.com/multiformats/go-multihash"
)

// Hash of Entry's Content
type Hash struct {
	H Multihash
}

// NewHash builds a Hash from a b58 string encoded hash
func NewHash(s string) (h Hash, err error) {
	h.H, err = FromB58String(s)
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
func (h *Hash) Sum(hc *Holochain, data []byte) (err error) {
	h.H, err = Sum(data, hc.hashCode, hc.hashLength)
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
