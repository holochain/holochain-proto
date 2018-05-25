// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Hash type for Holochains
// Holochain hashes are SHA256 binary values encoded to strings as base58

package hash

import (
	"encoding/binary"
	peer "github.com/libp2p/go-libp2p-peer"
	mh "github.com/multiformats/go-multihash"
	"io"
	"math/big"
	"sort"
)

// Hash of Entry's Content
type Hash string

// HashSpec holds the info that tells what kind of hash this is
type HashSpec struct {
	Code   uint64
	Length int
}

// NewHash builds a Hash from a b58 string encoded hash
func NewHash(s string) (h Hash, err error) {
	var multiH mh.Multihash
	multiH, err = mh.FromB58String(s)
	h = Hash(multiH)
	return
}

// HashFromBytes cast a byte slice to Hash type, and validate
// the id to make sure it is a multihash.
func HashFromBytes(b []byte) (h Hash, err error) {
	var multiH mh.Multihash
	if multiH, err = mh.Cast(b); err != nil {
		h = NullHash()
		return
	}
	h = Hash(multiH)
	return
}

// HashFromPeerID copy the bytes from a peer ID to Hash.
// Hashes and peer ID's are the exact same format, a multihash.
// NOTE: assumes that the multihash is valid
func HashFromPeerID(id peer.ID) (h Hash) {
	return Hash(id)
}

// PeerIDFromHash copy the bytes from a hash to a peer ID.
// Hashes and peer ID's are the exact same format, a multihash.
// NOTE: assumes that the multihash is valid
func PeerIDFromHash(h Hash) (id peer.ID) {
	id = peer.ID(h)
	return
}

// String encodes a hash to a human readable string
func (h Hash) String() string {
	//if cap(h) == 0 {
	//	return ""
	//}
	return mh.Multihash(h).B58String()
}

// Sum builds a digest according to the specs in the Holochain
func Sum(hc HashSpec, data []byte) (hash Hash, err error) {
	var multiH mh.Multihash
	multiH, err = mh.Sum(data, hc.Code, hc.Length)
	hash = Hash(multiH)
	return
}

// IsNullHash checks to see if this hash's value is the null hash
func (h Hash) IsNullHash() bool {
	return h == ""
	//return cap(h) == 1 && h[0] == 0
}

// NullHash builds a null valued hash
func NullHash() (h Hash) {
	//	null := [1]byte{0}
	//	h = null[:]
	return ""
}

// Clone returns a copy of a hash
func (h Hash) Clone() (hash Hash) {
	//	hash = make([]byte, len(h))
	//	copy(hash, h)
	hash = h
	return
}

// Equal checks to see if two hashes have the same value
func (h Hash) Equal(h2 Hash) bool {
	if h.IsNullHash() && h2.IsNullHash() {
		return true
	}
	return h == h2
}

// MarshalHash writes a hash to a binary stream
func (h Hash) MarshalHash(writer io.Writer) (err error) {
	if h.IsNullHash() {
		b := make([]byte, 34)
		err = binary.Write(writer, binary.LittleEndian, b)
	} else {
		err = binary.Write(writer, binary.LittleEndian, []byte(h))
	}
	return
}

// UnmarshalHash reads a hash from a binary stream
func UnmarshalHash(reader io.Reader) (hash Hash, err error) {
	b := make([]byte, 34)
	err = binary.Read(reader, binary.LittleEndian, b)
	if err == nil {
		if b[0] == 0 {
			hash = NullHash()
		} else {
			hash = Hash(b)
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
	k3 := XOR([]byte(h1), []byte(h2))

	// interpret it as an integer
	dist := big.NewInt(0).SetBytes(k3)
	return dist
}

// Less returns whether the first key is smaller than the second.
func HashLess(h1, h2 Hash) bool {
	a := h1
	b := h2
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
