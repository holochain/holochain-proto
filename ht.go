// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements an abstraction for holding hash table entries
// the DHT is built on top of this abstraction

package holochain

import (
	"errors"
	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
)

const (
	// constants for status action type

	AddLinkAction = ""
	DelLinkAction = "d"

	// constants for the state of the data, they are bit flags

	StatusDefault  = 0x00
	StatusLive     = 0x01
	StatusRejected = 0x02
	StatusDeleted  = 0x04
	StatusModified = 0x08
	StatusAny      = 0xFF

	// constants for the stored string status values in buntdb and for building code

	StatusLiveVal     = "1"
	StatusRejectedVal = "2"
	StatusDeletedVal  = "4"
	StatusModifiedVal = "8"
	StatusAnyVal      = "255"

	// constants for system reseved tags (start with 2 underscores)

	SysTagReplacedBy = "__replacedBy"

	// constants for get request GetMask

	GetMaskDefault   = 0x00
	GetMaskEntry     = 0x01
	GetMaskEntryType = 0x02
	GetMaskSources   = 0x04
	GetMaskAll       = 0xFF

	// constants for building code for GetMask

	GetMaskDefaultStr   = "0"
	GetMaskEntryStr     = "1"
	GetMaskEntryTypeStr = "2"
	GetMaskSourcesStr   = "4"
	GetMaskAllStr       = "255"
)

const (
	ReceiptOK = iota
	ReceiptRejected
)

// TaggedHash holds associated entries for the LinkQueryResponse
type TaggedHash struct {
	H         string // the hash of the link; gets filled by dht base node when answering get link request
	E         string // the value of link, gets filled if options set Load to true
	EntryType string // the entry type of the link, gets filled if options set Load to true
	T         string // the tag of the link, gets filled only if a tag wasn't specified and all tags are being returns
	Source    string // the statuses on the link, gets filled if options set Load to true
}

var ErrLinkNotFound = errors.New("link not found")
var ErrPutLinkOverDeleted = errors.New("putlink over deleted link")
var ErrHashDeleted = errors.New("hash deleted")
var ErrHashModified = errors.New("hash modified")
var ErrHashRejected = errors.New("hash rejected")
var ErrEntryTypeMismatch = errors.New("entry type mismatch")

type HashTableIterateFn func(hash Hash) (stop bool)

// HashTable provides an abstraction for storing the necessary DHT data
type HashTable interface {

	// Open initializes the table
	Open(options interface{}) (err error)

	// Close cleans up any resources used by the table
	Close()

	// Put stores a value to the DHT store
	Put(msg *Message, entryType string, key Hash, src peer.ID, value []byte, status int) (err error)

	// Del moves the given hash to the StatusDeleted status
	Del(msg *Message, key Hash) (err error)

	// Mod moves the given hash to the StatusModified status
	Mod(msg *Message, key Hash, newkey Hash) (err error)

	// Exists checks for the existence of the hash in the table
	Exists(key Hash, statusMask int) (err error)

	// Source returns the source node address of a given hash
	Source(key Hash) (id peer.ID, err error)

	// Get retrieves a value from the DHT store
	Get(key Hash, statusMask int, getMask int) (data []byte, entryType string, sources []string, status int, err error)

	// PutLink associates a link with a stored hash
	PutLink(m *Message, base string, link string, tag string) (err error)

	// DelLink removes a link and tag associated with a stored hash
	DelLink(m *Message, base string, link string, tag string) (err error)

	// GetLinks retrieves meta value associated with a base
	GetLinks(base Hash, tag string, statusMask int) (results []TaggedHash, err error)

	// GetIdx returns the current index of changes to the HashTable
	GetIdx() (idx int, err error)

	// GetIdxMessage returns the messages that causes the change at a given index
	GetIdxMessage(idx int) (msg Message, err error)

	// String converts the table into a human readable string
	String() string

	// JSON converts a DHT into a JSON string representation.
	JSON() (result string, err error)

	// Iterate call fn on all the hashes in the table
	Iterate(fn HashTableIterateFn)

	// GetReceipts returns a list of receipts that were generated regarding a hash
	//GetReceipts()
}
