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
	. "github.com/metacurrency/holochain/hash"
	"io"
	"os"
	"sync"
	"time"
)

// WalkerFn a function type for call Walk
type WalkerFn func(key *Hash, header *Header, entry Entry) error

var ErrHashNotFound = errors.New("hash not found")
var ErrIncompleteChain = errors.New("operation not allowed on incomplete chain")

const (
	ChainMarshalFlagsNone            = 0x00
	ChainMarshalFlagsNoHeaders       = 0x01
	ChainMarshalFlagsNoEntries       = 0x02
	ChainMarshalFlagsOmitDNA         = 0x04
	ChainMarshalFlagsNoPrivate       = 0x08
	ChainMarshalPrivateEntryRedacted = "%%PRIVATE ENTRY REDACTED%%"
)

// Chain structure for providing in-memory access to chain data, entries headers and hashes
type Chain struct {
	Hashes   []Hash
	Headers  []*Header
	Entries  []Entry
	TypeTops map[string]int // pointer to index of top of a given type
	Hmap     map[string]int // map header hashes to index number
	Emap     map[string]int // map entry hashes to index number

	//---

	s        *os.File // if this stream is not nil, new entries will get marshaled to it
	hashSpec HashSpec
	lk       sync.RWMutex
}

// NewChain creates and empty chain
func NewChain(hashSpec HashSpec) (chain *Chain) {
	c := Chain{
		Headers:  make([]*Header, 0),
		Entries:  make([]Entry, 0),
		Hashes:   make([]Hash, 0),
		TypeTops: make(map[string]int),
		Hmap:     make(map[string]int),
		Emap:     make(map[string]int),
		hashSpec: hashSpec,
	}
	chain = &c
	return
}

// NewChainFromFile creates a chain from a file, loading any data there,
// and setting it to be persisted to. If no file exists it will be created.
func NewChainFromFile(spec HashSpec, path string) (c *Chain, err error) {
	defer func() {
		if err != nil {
			Debugf("error loading chain :%s", err.Error())
		}
	}()
	c = NewChain(spec)

	var f *os.File
	if FileExists(path) {
		f, err = os.Open(path)
		if err != nil {
			return
		}
		var i int
		for {
			var header *Header
			var e Entry
			header, e, err = readPair(ChainMarshalFlagsNone, f)
			if err != nil && err.Error() == "EOF" {
				err = nil
				break
			}
			if err != nil {
				Debugf("error reading pair:%s", err.Error())
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
			hash, _, err = hd.Sum(spec)
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
	return c.Nth(0)
}

// Nth returns the nth latest header
func (c *Chain) Nth(n int) (header *Header) {
	c.lk.RLock()
	defer c.lk.RUnlock()
	l := len(c.Headers)
	if l-n > 0 {
		header = c.Headers[l-n-1]
	}
	return
}

// TopType returns the latest header of a given type
func (c *Chain) TopType(entryType string) (hash *Hash, header *Header) {
	c.lk.RLock()
	defer c.lk.RUnlock()
	i, ok := c.TypeTops[entryType]
	if ok {
		header = c.Headers[i]
		var hs = c.Hashes[i].Clone()
		hash = &hs
	}
	return
}

// AddEntry creates a new header and adds it to a chain
func (c *Chain) AddEntry(now time.Time, entryType string, e Entry, privKey ic.PrivKey) (hash Hash, err error) {
	c.lk.Lock()
	defer c.lk.Unlock()
	var l int
	var header *Header
	now = now.Round(0)
	l, hash, header, err = c.prepareHeader(now, entryType, e, privKey, nil)
	if err == nil {
		err = c.addEntry(l, hash, header, e)
	}
	return
}

// prepareHeader builds a header that could be added to the chain.
// Not thread safe, this must be called with the chain locked for writing so something else
// doesn't get inserted
func (c *Chain) prepareHeader(now time.Time, entryType string, e Entry, privKey ic.PrivKey, change *StatusChange) (entryIdx int, hash Hash, header *Header, err error) {
	// get the previous hashes
	var ph, pth Hash

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

	hash, header, err = newHeader(c.hashSpec, now, entryType, e, privKey, ph, pth, change)
	if err != nil {
		return
	}
	entryIdx = l
	return
}

// addEntry, low level entry add, not thread safe, must call c.lock in the calling funciton
func (c *Chain) addEntry(entryIdx int, hash Hash, header *Header, e Entry) (err error) {

	l := len(c.Hashes)
	if l != entryIdx {
		err = errors.New("entry indexes don't match can't create new entry")
		return
	}

	if l != len(c.Entries) {
		err = ErrIncompleteChain
		return
	}

	var g GobEntry
	g = *e.(*GobEntry)

	c.Hashes = append(c.Hashes, hash)
	c.Headers = append(c.Headers, header)
	c.Entries = append(c.Entries, &g)
	c.TypeTops[header.Type] = entryIdx
	c.Emap[header.EntryLink.String()] = entryIdx
	c.Hmap[hash.String()] = entryIdx

	if c.s != nil {
		err = writePair(c.s, header, &g)
	}

	return
}

// Get returns the header of a given hash
func (c *Chain) Get(h Hash) (header *Header, err error) {
	c.lk.RLock()
	defer c.lk.RUnlock()
	i, ok := c.Hmap[h.String()]
	if ok {
		header = c.Headers[i]
	} else {
		err = ErrHashNotFound
	}
	return
}

// GetEntry returns the entry of a given entry hash
func (c *Chain) GetEntry(h Hash) (entry Entry, entryType string, err error) {
	c.lk.RLock()
	defer c.lk.RUnlock()
	i, ok := c.Emap[h.String()]
	if ok {
		entry = c.Entries[i]
		entryType = c.Headers[i].Type
	} else {
		err = ErrHashNotFound
	}
	return
}

// GetEntryHeader returns the header of a given entry hash
func (c *Chain) GetEntryHeader(h Hash) (header *Header, err error) {
	c.lk.RLock()
	defer c.lk.RUnlock()
	i, ok := c.Emap[h.String()]
	if ok {
		header = c.Headers[i]
	} else {
		err = ErrHashNotFound
	}
	return
}

func writePair(writer io.Writer, header *Header, entry Entry) (err error) {
	if header != nil {
		err = MarshalHeader(writer, header)
		if err != nil {
			return
		}
	}
	if entry != nil {
		err = MarshalEntry(writer, entry)
	}
	return
}

func readPair(flags int64, reader io.Reader) (header *Header, entry Entry, err error) {
	if (flags & ChainMarshalFlagsNoHeaders) == 0 {
		var hd Header
		err = UnmarshalHeader(reader, &hd, 34)
		if err != nil {
			return
		}
		header = &hd
	}
	if (flags & ChainMarshalFlagsNoEntries) == 0 {
		entry, err = UnmarshalEntry(reader)
	}
	return
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func filterPass(index int, header *Header, whitelistTypes []string, blacklistTypes []string) bool {
	pass := true
	if len(whitelistTypes) > 0 {
		pass = contains(whitelistTypes, header.Type)
	}

	if len(blacklistTypes) > 0 {
		pass = pass && !contains(blacklistTypes, header.Type)
	}
	return pass
}

type ChainPair struct {
	Header *Header
	Entry  Entry
}

// MarshalChain serializes a chain data to a writer
func (c *Chain) MarshalChain(writer io.Writer, flags int64, whitelistTypes []string, privateTypes []string) (err error) {
	c.lk.RLock()
	defer c.lk.RUnlock()

	if len(c.Headers) != len(c.Entries) {
		err = ErrIncompleteChain
		return
	}

	err = binary.Write(writer, binary.LittleEndian, flags)
	if err != nil {
		return err
	}

	var pairsToWrite []ChainPair
	var lastHeaderToWrite int

	for i, hdr := range c.Headers {
		var empty []string
		var e Entry

		if i == 0 || filterPass(i, hdr, whitelistTypes, empty) {
			e = c.Entries[i]

			if (i == 0) && ((flags & ChainMarshalFlagsOmitDNA) != 0) {
				e = &GobEntry{C: ""}
			}

			if (flags & ChainMarshalFlagsNoEntries) != 0 {
				e = nil
			}

			if !filterPass(i, hdr, whitelistTypes, privateTypes) {
				e = &GobEntry{C: ChainMarshalPrivateEntryRedacted}
			}

			if (flags & ChainMarshalFlagsNoHeaders) != 0 {
				hdr = nil
			} else {
				lastHeaderToWrite = i
			}

			pairsToWrite = append(pairsToWrite, ChainPair{Header: hdr, Entry: e})
		}
	}

	err = binary.Write(writer, binary.LittleEndian, int64(len(pairsToWrite)))
	if err != nil {
		return err
	}

	for _, pair := range pairsToWrite {
		err = writePair(writer, pair.Header, pair.Entry)
		if err != nil {
			return
		}
	}

	if (flags & ChainMarshalFlagsNoHeaders) == 0 {
		hash := c.Hashes[lastHeaderToWrite]
		err = hash.MarshalHash(writer)
	}
	return
}

// addPair adds header and entry pairs to the chain during unmarshaling
// This call assumes that Hashes array is one element behind the Headers and Entries
// because for each pair (except the 0th) it adds the hash of the previous entry
// thus it also means that you must add the last Hash after you have finished calling addPair
func (c *Chain) addPair(header *Header, entry Entry, i int) {
	if header != nil {
		if i > 0 {
			s := header.HeaderLink.String()
			h, _ := NewHash(s)
			c.Hashes = append(c.Hashes, h)
			c.Hmap[s] = i - 1
		}
		c.Headers = append(c.Headers, header)
		c.TypeTops[header.Type] = i
		c.Emap[header.EntryLink.String()] = i
	}
	if entry != nil {
		c.Entries = append(c.Entries, entry)
	}
}

// UnmarshalChain unserializes a chain from a reader
func UnmarshalChain(hashSpec HashSpec, reader io.Reader) (flags int64, c *Chain, err error) {
	defer func() {
		if err != nil {
			Debugf("error unmarshaling chain:%s", err.Error())
		}
	}()
	c = NewChain(hashSpec)
	err = binary.Read(reader, binary.LittleEndian, &flags)
	if err != nil {
		return
	}
	var l, i int64
	err = binary.Read(reader, binary.LittleEndian, &l)
	if err != nil {
		return
	}
	for i = 0; i < l; i++ {
		var header *Header
		var e Entry
		header, e, err = readPair(flags, reader)
		if err != nil {
			return
		}
		c.addPair(header, e, int(i))
	}

	if (flags & ChainMarshalFlagsNoHeaders) == 0 {
		// decode final hash
		var h Hash
		err = h.UnmarshalHash(reader)
		if err != nil {
			return
		}
		c.Hashes = append(c.Hashes, h)
		c.Hmap[h.String()] = int(i - 1)
	}
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
func (c *Chain) Validate(skipEntries bool) (err error) {
	c.lk.RLock()
	defer c.lk.RUnlock()
	l := len(c.Headers)
	for i := 0; i < l; i++ {
		hd := c.Headers[i]

		var hash, nexth Hash
		// hash the header
		hash, _, err = hd.Sum(c.hashSpec)
		if err != nil {
			return
		}
		// we can't compare top hash to next link, because it doesn't exist yet!
		if i < l-2 {
			nexth = c.Headers[i+1].HeaderLink
		} else {
			// so get it from the Hashes (even though this could be cheated)
			nexth = c.Hashes[i]
		}

		if !hash.Equal(&nexth) {
			err = fmt.Errorf("header hash mismatch at link %d", i)
			return
		}

		if !skipEntries {
			var b []byte
			b, err = c.Entries[i].Marshal()
			if err != nil {
				return
			}
			err = hash.Sum(c.hashSpec, b)
			if err != nil {
				return
			}

			if !bytes.Equal(hash.H, hd.EntryLink.H) {
				err = fmt.Errorf("entry hash mismatch at link %d", i)
				return
			}
		}
	}
	return
}

// String converts a chain to a textual dump of the headers and entries
func (c *Chain) String() string {
	c.lk.RLock()
	defer c.lk.RUnlock()
	l := len(c.Headers)
	r := ""
	for i := 0; i < l; i++ {
		hdr := c.Headers[i]
		hash := c.Hashes[i]
		r += fmt.Sprintf("%s:%s @ %v\n", hdr.Type, hash, hdr.Time)
		r += fmt.Sprintf("    Next Header: %v\n", hdr.HeaderLink)
		r += fmt.Sprintf("    Next %s: %v\n", hdr.Type, hdr.TypeLink)
		r += fmt.Sprintf("    Entry: %v\n", hdr.EntryLink)
		e := c.Entries[i]
		switch hdr.Type {
		case KeyEntryType:
			r += fmt.Sprintf("       %v\n", e.(*GobEntry).C)
		case DNAEntryType:
			r += fmt.Sprintf("       %s\n", e.(*GobEntry).C)
		case AgentEntryType:
			r += fmt.Sprintf("       %v\n", e.(*GobEntry).C)
		default:
			r += fmt.Sprintf("       %v\n", e)
		}
		r += "\n"
	}
	return r
}

// Length returns the number of entries in the chain
func (c *Chain) Length() int {
	return len(c.Headers)
}

// Close the chain's file
func (c *Chain) Close() {
	c.s.Close()
	c.s = nil
}
