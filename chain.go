// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements in-memory chain representation with marshaling, & validation

package holochain

import (
	"encoding/binary"
	"encoding/gob"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	"io"
	"time"
)

// Chain structure for providing in-memory access to chain data, entries headers and hashes
type Chain struct {
	Hashes   []Hash
	Headers  []*Header
	Entries  []Entry
	TypeTops map[string]int // pointer to index of top of a given type
	Hmap     map[string]int // map header hashes to index number
	Emap     map[string]int // map entry hashes to index number
}

type Signature struct {
	S []byte
}

// Header holds chain links, type, timestamp and signature
type Header struct {
	Type       string
	Time       time.Time
	HeaderLink Hash // link to previous header
	EntryLink  Hash // link to entry
	TypeLink   Hash // link to header of previous header of this type
	Sig        Signature
	Meta       interface{}
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

// newHeader makes Header object linked to a previous Header by hash
func newHeader(h *Holochain, now time.Time, t string, entry Entry, key ic.PrivKey, prev Hash, prevType Hash) (hash Hash, header *Header, err error) {
	var hd Header
	hd.Type = t
	hd.Time = now
	hd.HeaderLink = prev
	hd.TypeLink = prevType

	// encode the entry into bytes
	m, err := entry.Marshal()
	if err != nil {
		return
	}

	// calculate the entry's hash and store it in the header
	err = hd.EntryLink.Sum(h, m)
	if err != nil {
		return
	}

	// sign the hash of the entry
	sig, err := key.Sign(hd.EntryLink.H)
	if err != nil {
		return
	}
	hd.Sig = Signature{S: sig}

	// encode the header and create a hash of it
	b, err := ByteEncoder(&hd)
	if err != nil {
		return
	}
	err = hash.Sum(h, b)
	if err != nil {
		return
	}

	header = &hd
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
func (c *Chain) AddEntry(h *Holochain, now time.Time, entryType string, e Entry, key ic.PrivKey) (hash Hash, err error) {

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
	return
}

// Get returns the header of a given hash
func (c *Chain) Get(h Hash) (header *Header) {
	i, ok := c.Hmap[h.String()]
	if ok {
		header = c.Headers[i]
	}
	return
}

// MarshalChain serializes a chain data to a writer
func (c *Chain) MarshalChain(writer io.Writer) (err error) {

	var l = uint64(len(c.Headers))
	err = binary.Write(writer, binary.LittleEndian, l)
	if err != nil {
		return err
	}

	enc := gob.NewEncoder(writer)

	for i, h := range c.Headers {
		err = enc.Encode(h)
		if err != nil {
			return
		}
		e := c.Entries[i]

		err = enc.Encode(&e.(*GobEntry).C)
		if err != nil {
			return
		}
	}

	hash := c.Hashes[l-1]
	err = enc.Encode(&hash)

	return
}

// UnmarshalChain unserializes a chain from a reader
func UnmarshalChain(reader io.Reader) (c *Chain, err error) {
	c = NewChain()
	var l, i uint64
	err = binary.Read(reader, binary.LittleEndian, &l)
	if err != nil {
		return
	}
	dec := gob.NewDecoder(reader)
	for i = 0; i < l; i++ {
		var header Header
		err = dec.Decode(&header)
		if err != nil {
			return
		}
		var e GobEntry
		err = dec.Decode(&e.C)
		if err != nil {
			return
		}
		if i > 0 {
			s := header.HeaderLink.String()
			h, _ := NewHash(s)
			c.Hashes = append(c.Hashes, h)
			c.Hmap[s] = int(i - 1)
		}
		c.Headers = append(c.Headers, &header)
		c.Entries = append(c.Entries, &e)
		c.TypeTops[header.Type] = int(i)
		c.Emap[header.EntryLink.String()] = int(i)
	}
	// decode final hash
	var h Hash
	err = dec.Decode(&h)
	if err != nil {
		return
	}
	c.Hashes = append(c.Hashes, h)
	c.Hmap[h.String()] = int(i - 1)
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
