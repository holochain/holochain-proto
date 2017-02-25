// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements in-memory chain representation with marshaling, & validation

package holochain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	"io"
	"os"
	"time"
)

// WalkerFn a function type for call Walk
type WalkerFn func(key *Hash, header *Header, entry Entry) error

var ErrHashNotFound error = errors.New("hash not found")

// Chain structure for providing in-memory access to chain data, entries headers and hashes
type Chain struct {
	Hashes   []Hash
	Headers  []*Header
	Entries  []Entry
	TypeTops map[string]int // pointer to index of top of a given type
	Hmap     map[string]int // map header hashes to index number
	Emap     map[string]int // map entry hashes to index number

	//---

	s *os.File // if this stream is not nil, new entries will get marshaled to it
}

// NewChain creates and empty chain
func NewChain() (chain *Chain) {
	c := Chain{
		Headers:  make([]*Header, 0),
		Entries:  make([]Entry, 0),
		Hashes:   make([]Hash, 0),
		TypeTops: make(map[string]int),
		Hmap:     make(map[string]int),
		Emap:     make(map[string]int),
	}
	chain = &c
	return
}

// Creates a chain from a file, loading any data there, and setting it to be persisted to
func NewChainFromFile(h HashSpec, path string) (c *Chain, err error) {
	c = NewChain()

	var f *os.File
	if fileExists(path) {
		f, err = os.Open(path)
		if err != nil {
			return
		}
		var i int
		for {
			var header *Header
			var e Entry
			header, e, err = readPair(f)
			if err != nil && err.Error() == "EOF" {
				err = nil
				break
			}
			if err != nil {
				return
			}
			c.addPair(header, e, i)
			i++
		}
		f.Close()
		i--
		// if we read anything then we have to calculate the final hash and add it
		if i >= 0 {
			hd := c.Headers[i]
			var hash Hash

			// hash the header
			hash, _, err = hd.Sum(h)
			if err != nil {
				return
			}

			c.Hashes = append(c.Hashes, hash)
			c.Hmap[hash.String()] = i

			// finally validate that it all hashes out correctly
			/*			err = c.Validate(h)
						if err != nil {
							return
						}
			*/
		}

		f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return
		}
	} else {
		f, err = os.Create(path)
		if err != nil {
			return
		}
	}
	c.s = f
	return
}

// Top returns the latest header
func (c *Chain) Top() (header *Header) {
	l := len(c.Headers)
	if l > 0 {
		header = c.Headers[l-1]
	}
	return
}

// TopType returns the latest header of a given type
func (c *Chain) TopType(entryType string) (header *Header) {
	i, ok := c.TypeTops[entryType]
	if ok {
		header = c.Headers[i]
	}
	return
}

// AddEntry creates a new header and adds it to a chain
func (c *Chain) AddEntry(h HashSpec, now time.Time, entryType string, e Entry, key ic.PrivKey) (hash Hash, err error) {

	// get the previous hashes
	var ph, pth Hash

	//@TODO make this transactional
	l := len(c.Hashes)
	if l == 0 {
		ph = NullHash()
	} else {
		ph = c.Hashes[l-1]
	}

	i, ok := c.TypeTops[entryType]
	if !ok {
		pth = NullHash()
	} else {
		pth = c.Hashes[i]
	}

	var g GobEntry
	g = *e.(*GobEntry)
	hash, header, err := newHeader(h, now, entryType, e, key, ph, pth)
	c.Hashes = append(c.Hashes, hash)
	c.Headers = append(c.Headers, header)
	c.Entries = append(c.Entries, &g)
	c.TypeTops[entryType] = l
	c.Emap[header.EntryLink.String()] = l
	c.Hmap[hash.String()] = l

	if c.s != nil {
		err = writePair(c.s, header, &g)
	}

	return
}

// Get returns the header of a given hash
func (c *Chain) Get(h Hash) (header *Header, err error) {
	i, ok := c.Hmap[h.String()]
	if ok {
		header = c.Headers[i]
	} else {
		err = ErrHashNotFound
	}
	return
}

// GetEntryHeader returns the header of a given entyr hash
func (c *Chain) GetEntryHeader(h Hash) (header *Header, err error) {
	i, ok := c.Emap[h.String()]
	if ok {
		header = c.Headers[i]
	} else {
		err = ErrHashNotFound
	}
	return
}

func writePair(writer io.Writer, header *Header, entry Entry) (err error) {
	err = MarshalHeader(writer, header)
	if err != nil {
		return
	}
	err = MarshalEntry(writer, entry)
	return
}

func readPair(reader io.Reader) (header *Header, entry Entry, err error) {
	var hd Header
	err = UnmarshalHeader(reader, &hd, 34)
	if err != nil {
		return
	}
	header = &hd
	entry, err = UnmarshalEntry(reader)
	return
}

// MarshalChain serializes a chain data to a writer
func (c *Chain) MarshalChain(writer io.Writer) (err error) {

	var l = uint64(len(c.Headers))
	err = binary.Write(writer, binary.LittleEndian, l)
	if err != nil {
		return err
	}

	for i, h := range c.Headers {
		e := c.Entries[i]
		err = writePair(writer, h, e)
		if err != nil {
			return
		}
	}

	hash := c.Hashes[l-1]
	err = hash.MarshalHash(writer)

	return
}

// addPair adds header and entry pairs to the chain during unmarshaling
// This call assumes that Hashes array is one element behind the Headers and Entries
// because for each pair (except the 0th) it adds the hash of the previous entry
// thus it also means that you must add the last Hash after you have finished calling
// addPair
func (c *Chain) addPair(header *Header, entry Entry, i int) {
	if i > 0 {
		s := header.HeaderLink.String()
		h, _ := NewHash(s)
		c.Hashes = append(c.Hashes, h)
		c.Hmap[s] = i - 1
	}
	c.Headers = append(c.Headers, header)
	c.Entries = append(c.Entries, entry)
	c.TypeTops[header.Type] = i
	c.Emap[header.EntryLink.String()] = i
}

// UnmarshalChain unserializes a chain from a reader
func UnmarshalChain(reader io.Reader) (c *Chain, err error) {
	c = NewChain()
	var l, i uint64
	err = binary.Read(reader, binary.LittleEndian, &l)
	if err != nil {
		return
	}
	for i = 0; i < l; i++ {
		var header *Header
		var e Entry
		header, e, err = readPair(reader)
		if err != nil {
			return
		}
		c.addPair(header, e, int(i))
	}
	// decode final hash
	var h Hash
	err = h.UnmarshalHash(reader)
	if err != nil {
		return
	}
	c.Hashes = append(c.Hashes, h)
	c.Hmap[h.String()] = int(i - 1)
	return
}

// Walk traverses chain from most recent to first entry calling fn on each one
func (c *Chain) Walk(fn WalkerFn) (err error) {
	l := len(c.Headers)
	for i := l - 1; i >= 0; i-- {
		err = fn(&c.Hashes[i], c.Headers[i], c.Entries[i])
		if err != nil {
			return
		}
	}
	return
}

// Validate traverses chain confirming the hashes
// @TODO confirm that TypeLinks are also correct
// @TODO confirm signatures
func (c *Chain) Validate(h HashSpec) (err error) {
	l := len(c.Headers)
	for i := l - 1; i >= 0; i-- {
		hd := c.Headers[i]
		var hash Hash

		// hash the header
		hash, _, err = hd.Sum(h)
		if err != nil {
			return
		}

		var nexth Hash
		if i == l-1 {
			nexth = c.Hashes[i]
		} else {
			nexth = c.Headers[i+1].HeaderLink
		}

		if !bytes.Equal(hash.H, nexth.H) {
			err = fmt.Errorf("header hash mismatch at link %d", i)
			return
		}

		var b []byte
		b, err = c.Entries[i].Marshal()
		if err != nil {
			return
		}
		err = hash.Sum(h, b)
		if err != nil {
			return
		}

		if !bytes.Equal(hash.H, hd.EntryLink.H) {
			err = fmt.Errorf("entry hash mismatch at link %d", i)
			return
		}
	}
	return
}

// String converts a chain to a textual dump of the headers and entries
func (c *Chain) String() string {
	l := len(c.Headers)
	r := ""
	for i := 0; i < l; i++ {
		r += fmt.Sprintf("Header:%v\n", *c.Headers[i])
		r += fmt.Sprintf("Entry:%v\n\n", c.Entries[i])
	}
	r += "Hashlist:\n"
	for i := 0; i < len(c.Headers); i++ {
		r += fmt.Sprintf("%s\n", c.Hashes[i].String())

	}
	return r
}
