// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Hash type for Holochains
// Holochain hashes are SHA256 binary values encoded to strings as base58

package hash

import (
	"bytes"
	"encoding/binary"
	"errors"
	peer "github.com/libp2p/go-libp2p-peer"
	mh "github.com/multiformats/go-multihash"
	"io"
	"math/big"
	"sort"
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

// HashFromBytes cast a byte slice to Hash type, and validate
// the id to make sure it is a multihash.
func HashFromBytes(b []byte) (h Hash, err error) {
	if h.H, err = mh.Cast(b); err != nil {
		h = NullHash()
		return
	}
	return
}

// HashFromPeerID copy the bytes from a peer ID to Hash.
// Hashes and peer ID's are the exact same format, a multihash.
// NOTE: assumes that the multihash is valid
func HashFromPeerID(id peer.ID) (h Hash) {
	h.H = make([]byte, len(id))
	copy(h.H, id)
	return
}

// PeerIDFromHash copy the bytes from a hash to a peer ID.
// Hashes and peer ID's are the exact same format, a multihash.
// NOTE: assumes that the multihash is valid
func PeerIDFromHash(h Hash) (id peer.ID) {
	id = peer.ID(h.H)
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

// XOR takes two byte slices, XORs them together, returns the resulting slice.
// taken from https://github.com/ipfs/go-ipfs-util/blob/master/util.go
func XOR(a, b []byte) []byte {
	c := make([]byte, len(a))
	for i := 0; i < len(a); i++ {
		c[i] = a[i] ^ b[i]
	}
	return c
}

// The code below is adapted from https://github.com/libp2p/go-libp2p-kbucket

// Distance returns the distance metric between two hashes
func HashXORDistance(h1, h2 Hash) *big.Int {
	// XOR the hashes
	k3 := XOR(h1.H, h2.H)

	// interpret it as an integer
	dist := big.NewInt(0).SetBytes(k3)
	return dist
}

// Less returns whether the first key is smaller than the second.
func HashLess(h1, h2 Hash) bool {
	a := h1.H
	b := h2.H
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return true
}

// ZeroPrefixLen returns the number of consecutive zeroes in a byte slice.
func ZeroPrefixLen(id []byte) int {
	for i := 0; i < len(id); i++ {
		for j := 0; j < 8; j++ {
			if (id[i]>>uint8(7-j))&0x1 != 0 {
				return i*8 + j
			}
		}
	}
	return len(id) * 8
}

// hashDistance helper struct for sorting by distance which pre-caches the distance
// to center so as not to recalculate it on every sort comparison.
type HashDistance struct {
	Hash     interface{}
	Distance *big.Int
}

type HashSorterArr []*HashDistance

func (p HashSorterArr) Len() int      { return len(p) }
func (p HashSorterArr) Swap(a, b int) { p[a], p[b] = p[b], p[a] }
func (p HashSorterArr) Less(a, b int) bool {
	return p[a].Distance.Cmp(p[b].Distance) == -1
}

// SortByDistance takes a center Hash, and a list of Hashes toSort.
// It returns a new list, where the Hashes toSort have been sorted by their
// distance to the center Hash.
func SortByDistance(center Hash, toSort []Hash) []Hash {
	var hsarr HashSorterArr
	for _, h := range toSort {
		hd := &HashDistance{
			Hash:     h,
			Distance: HashXORDistance(h, center),
		}
		hsarr = append(hsarr, hd)
	}
	sort.Sort(hsarr)
	var out []Hash
	for _, hd := range hsarr {
		out = append(out, hd.Hash.(Hash))
	}
	return out
}
