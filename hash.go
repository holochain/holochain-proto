// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file

// Hash type for Holochains
// Holochain hashes are SHA256 binary values encoded to strings as base58
package holochain

import (
	b58 "github.com/jbenet/go-base58"
)

// SHA256 hash of Entry's Content
type Hash [32]byte

// String encodes a hash to a human readable string
func (h Hash) String() string {
	return b58.Encode(h[:])
}

// NewHash builds a Hash from a string encoded hash
func NewHash(s string) (h Hash) {
	copy(h[:],b58.Decode(s))
	return
}
