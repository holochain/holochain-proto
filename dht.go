// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// DHT implements the distributed hash table

package holochain

import (
	"errors"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/tidwall/buntdb"
	"strconv"
	"strings"
)

// Holds the dht configuration options
type DHTConfig struct {
	// HashType : (string) Identifies hash type to be used for this application. Should be from the list of hash types from the multihash library
	HashType string

	// NeighborhoodSize : (integer) Establishes minimum online redundancy targets for data, and size of peer sets for sync gossip. A neighborhood size of ZERO means no sharding (every node syncs all data with every other node). ONE means you are running this as a centralized application and gossip is turned OFF. For most applications we recommend neighborhoods no smaller than 8 for nearness or 32 for hashmask sharding.

	// ShardingMethod : Identifier for sharding method (none, XOR, hashmask, other nearness algorithms?, etc.)

	// MaxLinkSets : (integer) Maximum number of results to return on a GetLinks query to keep computation and traffic to a reasonable size. You need to break these result sets into multiple "pages" of results retrieve more.

	// ValidationTimeout : (integer) Time period in seconds, until data that needs to be validated against a source remains "alive" to keep trying to get validation from that source. If someone commits something and then goes offline, how long do they have to come back online before DHT sync requests consider that data invalid?

	//PeerTimeout : (integer) Time period in seconds, until a node drops a peer from its neighborhood list for failing to respond to gossip requests.

	// WireEncryption : settings for point-to-point encryption of messages on the network (none, AES, what are the options?)

	// DataEncryption : What are the options for encrypting data at rest in the dht.db that don't break db functionality? Is there really a point to trying to do this?

	// MaxEntrySize : Sets the maximum allowable size of entries for this holochain
}

type gossipWithReq struct {
	id peer.ID
}

// DHT struct holds the data necessary to run the distributed hash table
type DHT struct {
	h         *Holochain // pointer to the holochain this DHT is part of
	db        *buntdb.DB
	puts      chan Message
	gossiping bool
	glog      *Logger // the gossip logger
	dlog      *Logger // the dht logger
	gossips   map[peer.ID]bool
	gchan     chan gossipWithReq
}

// Meta holds data that can be associated with a hash
// @todo, we should also be storing the meta-data source
type Meta struct {
	H Hash   // hash of link-data associated
	T string // meta-data type identifier
	V []byte // meta-data
}

const (
	// constants for status action type

	AddAction = ""
	ModAction = "m"
	DelAction = "d"

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

// PutReq holds the data of a put request
type PutReq struct {
	H Hash
	S int
	D interface{}
}

// GetReq holds the data of a get request
type GetReq struct {
	H          Hash
	StatusMask int
	GetMask    int
}

// GetResp holds the data of a get response
type GetResp struct {
	Entry      Entry
	EntryType  string
	Sources    []string
	FollowHash string // hash of new entry if the entry was modified and needs following
}

// DelReq holds the data of a del request
type DelReq struct {
	H  Hash // hash to be deleted
	By Hash // hash of DelEntry on source chain took this action
}

// ModReq holds the data of a mod request
type ModReq struct {
	H Hash
	N Hash
}

// LinkReq holds a link request
type LinkReq struct {
	Base  Hash // data on which to attach the links
	Links Hash // hash of the links entry
}

// DelLinkReq holds a delete link request
type DelLinkReq struct {
	Base Hash   // data on which to link was attached
	Link Hash   // hash of the link entry
	Tag  string // tag to be deleted
}

// LinkQuery holds a getLink query
type LinkQuery struct {
	Base       Hash
	T          string
	StatusMask int
	// order
	// filter, etc
}

// GetOptions options to holochain level Get functions
type GetOptions struct {
	StatusMask int  // mask of which status of entries to return
	GetMask    int  // mask of what to include in the response
	Local      bool // bool if get should happen from chain not DHT
}

// GetLinkOptions options to holochain level GetLink functions
type GetLinkOptions struct {
	Load       bool // indicates whether GetLink should retrieve the entries of all links
	StatusMask int  // mask of which status of links to return
}

// TaggedHash holds associated entries for the LinkQueryResponse
type TaggedHash struct {
	H string // the hash of the link; gets filled by dht base node when answering get link request
	E string // the value of link, get's filled by caller if getLink function set Load to true
}

// LinkQueryResp holds response to getLink query
type LinkQueryResp struct {
	Links []TaggedHash
}

var ErrLinkNotFound = errors.New("link not found")
var ErrHashDeleted = errors.New("hash deleted")
var ErrHashModified = errors.New("hash modified")
var ErrHashRejected = errors.New("hash rejected")

var ErrEntryTypeMismatch = errors.New("entry type mismatch")

// NewDHT creates a new DHT structure
func NewDHT(h *Holochain) *DHT {
	dht := DHT{
		h:    h,
		glog: &h.config.Loggers.Gossip,
		dlog: &h.config.Loggers.DHT,
	}
	db, err := buntdb.Open(h.DBPath() + "/" + DHTStoreFileName)
	if err != nil {
		panic(err)
	}
	db.CreateIndex("link", "link:*", buntdb.IndexString)
	db.CreateIndex("idx", "idx:*", buntdb.IndexInt)
	db.CreateIndex("peer", "peer:*", buntdb.IndexString)

	dht.db = db
	dht.puts = make(chan Message, 10)

	dht.gossips = make(map[peer.ID]bool)
	dht.gchan = make(chan gossipWithReq, 10)

	return &dht
}

// SetupDHT prepares a DHT for use by putting the genesis entries that are added by GenChain
func (dht *DHT) SetupDHT() (err error) {
	x := ""
	// put the holochain id so it always exists for linking
	dna := dht.h.DNAHash()
	err = dht.put(nil, DNAEntryType, dna, dht.h.nodeID, []byte(x), StatusLive)
	if err != nil {
		return
	}

	// put the KeyEntry so it always exists for retrieving the public key
	kh, err := NewHash(peer.IDB58Encode(dht.h.nodeID))
	if err != nil {
		return
	}
	if err = dht.put(dht.h.node.NewMessage(PUT_REQUEST, PutReq{H: kh}), KeyEntryType, kh, dht.h.nodeID, []byte(dht.h.nodeID), StatusLive); err != nil {
		return
	}

	// put the AgentEntry so it always exists for linking
	a := dht.h.AgentHash()
	var e Entry
	var t string
	e, t, err = dht.h.chain.GetEntry(a)
	if err != nil {
		return err
	}
	// sanity check
	if t != AgentEntryType {
		panic("bad type!!")
	}

	var b []byte
	b, err = e.Marshal()
	if err != nil {
		return
	}
	if err = dht.put(dht.h.node.NewMessage(PUT_REQUEST, PutReq{H: a}), AgentEntryType, a, dht.h.nodeID, b, StatusLive); err != nil {
		return
	}

	return
}

// put stores a value to the DHT store
// N.B. This call assumes that the value has already been validated
func (dht *DHT) put(m *Message, entryType string, key Hash, src peer.ID, value []byte, status int) (err error) {
	k := key.String()
	dht.dlog.Logf("put %s=>%s", k, string(value))
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		_, err := incIdx(tx, m)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("entry:"+k, string(value), nil)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("type:"+k, entryType, nil)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("src:"+k, peer.IDB58Encode(src), nil)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("status:"+k, fmt.Sprintf("%d", status), nil)
		if err != nil {
			return err
		}
		return err
	})
	return
}

func _setStatus(tx *buntdb.Tx, m *Message, key string, status int) (err error) {

	_, err = tx.Get("entry:" + key)
	if err != nil {
		if err == buntdb.ErrNotFound {
			err = ErrHashNotFound
		}
		return
	}

	_, err = incIdx(tx, m)
	if err != nil {
		return
	}

	_, _, err = tx.Set("status:"+key, fmt.Sprintf("%d", status), nil)
	if err != nil {
		return
	}
	return
}

// del moves the given hash to the StatusDeleted status
// N.B. this functions assumes that the validity of this action has been confirmed
func (dht *DHT) del(m *Message, key Hash) (err error) {
	k := key.String()
	dht.dlog.Logf("del %s", k)
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		err = _setStatus(tx, m, k, StatusDeleted)
		return err
	})
	return
}

// mod moves the given hash to the StatusModified status
// N.B. this functions assumes that the validity of this action has been confirmed
func (dht *DHT) mod(m *Message, key Hash, newkey Hash) (err error) {
	k := key.String()
	dht.dlog.Logf("mod %s", k)
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		err = _setStatus(tx, m, k, StatusModified)
		if err == nil {
			link := newkey.String()
			err = _putLink(tx, k, link, SysTagReplacedBy)
			if err == nil {
				_, _, err = tx.Set("replacedBy:"+k, link, nil)
				if err != nil {
					return err
				}
			}
		}
		return err
	})
	return
}

func _get(tx *buntdb.Tx, k string, statusMask int) (string, error) {
	val, err := tx.Get("entry:" + k)
	if err == buntdb.ErrNotFound {
		err = ErrHashNotFound
		return val, err
	}
	var statusVal string
	statusVal, err = tx.Get("status:" + k)
	if err == nil {

		if statusMask == StatusDefault {
			// if the status mask is not given (i.e. Default) then
			// we return information about the status if it's other than live
			switch statusVal {
			case StatusDeletedVal:
				err = ErrHashDeleted
			case StatusModifiedVal:
				val, err = tx.Get("replacedBy:" + k)
				if err != nil {
					panic("missing expected replacedBy record")
				}
				err = ErrHashModified
			case StatusRejectedVal:
				err = ErrHashRejected
			case StatusLiveVal:
			default:
				panic("unknown status!")
			}
		} else {
			// otherwise we return the value only if the status is in the mask
			var status int
			status, err = strconv.Atoi(statusVal)
			if err == nil {
				if (status & statusMask) == 0 {
					err = ErrHashNotFound
				}
			}
		}
	}
	return val, err
}

// exists checks for the existence of the hash in the store
func (dht *DHT) exists(key Hash, statusMask int) (err error) {
	err = dht.db.View(func(tx *buntdb.Tx) error {
		_, err := _get(tx, key.String(), statusMask)
		return err
	})
	return
}

// returns the source of a given hash
func (dht *DHT) source(key Hash) (id peer.ID, err error) {
	err = dht.db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("src:" + key.String())
		if err == buntdb.ErrNotFound {
			err = ErrHashNotFound
		}
		if err == nil {
			id, err = peer.IDB58Decode(val)
		}
		return err
	})
	return
}

// get retrieves a value from the DHT store
func (dht *DHT) get(key Hash, statusMask int, getMask int) (data []byte, entryType string, sources []string, status int, err error) {
	if getMask == GetMaskDefault {
		getMask = GetMaskEntry
	}
	err = dht.db.View(func(tx *buntdb.Tx) error {
		k := key.String()
		val, err := _get(tx, k, statusMask)
		if err != nil {
			data = []byte(val) // gotta do this because value is valid if ErrHashModified
			return err
		}
		data = []byte(val)

		if (getMask & GetMaskEntryType) != 0 {
			entryType, err = tx.Get("type:" + k)
			if err != nil {
				return err
			}
		}
		if (getMask & GetMaskSources) != 0 {
			val, err = tx.Get("src:" + k)
			if err == buntdb.ErrNotFound {
				err = ErrHashNotFound
			}
			if err == nil {
				sources = append(sources, val)
			}
			if err != nil {
				return err
			}
		}

		val, err = tx.Get("status:" + k)
		if err != nil {
			return err
		}
		status, err = strconv.Atoi(val)
		if err != nil {
			return err
		}

		return err
	})
	return
}

// _putLink is a low level routine to add a link, also used by mod
func _putLink(tx *buntdb.Tx, base string, link string, tag string) (err error) {
	key := "link:" + base + ":" + link + ":" + tag
	var val string
	val, err = tx.Get(key)
	if err == buntdb.ErrNotFound {
		_, _, err = tx.Set(key, StatusLiveVal, nil)
		if err != nil {
			return
		}
	} else {
		//TODO what do we do if there's already something there?
		//		if val != StatusLiveVal {
		Debugf("putlink when %v has status %v", key, val)
		panic("putLink over existing link not implemented")
		//		}
	}
	return
}

// putLink associates a link with a stored hash
// N.B. this function assumes that the data associated has been properly retrieved
// and validated from the cource chain
func (dht *DHT) putLink(m *Message, base string, link string, tag string) (err error) {
	dht.dlog.Logf("putLink on %v link %v as %s", base, link, tag)
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		_, err := _get(tx, base, StatusLive)
		if err != nil {
			return err
		}

		err = _putLink(tx, base, link, tag)
		if err != nil {
			return err
		}

		//var index string
		_, err = incIdx(tx, m)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

// delLink removes a link and tag associated with a stored hash
// N.B. this function assumes that the action has been properly validated
func (dht *DHT) delLink(m *Message, base string, link string, tag string) (err error) {
	dht.dlog.Logf("delLink on %v link %v as %s", base, link, tag)
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		_, err := _get(tx, base, StatusLive)
		if err != nil {
			return err
		}

		key := "link:" + base + ":" + link + ":" + tag
		val, err := tx.Get(key)
		if err == buntdb.ErrNotFound {
			return ErrLinkNotFound
		}
		if err != nil {
			return err
		}

		if val == StatusLiveVal {
			//var index string
			_, err = incIdx(tx, m)
			if err != nil {
				return err
			}
			_, _, err = tx.Set(key, StatusDeletedVal, nil)
			if err != nil {
				return err
			}

		} else {
			// TODO what do we do about deleting deleted links!?
			// ignore for now.
		}

		return err
	})
	return
}

func filter(ss []Meta, test func(*Meta) bool) (ret []Meta) {
	for _, s := range ss {
		if test(&s) {
			ret = append(ret, s)
		}
	}
	return
}

// getLink retrieves meta value associated with a base
func (dht *DHT) getLink(base Hash, tag string, statusMask int) (results []TaggedHash, err error) {
	dht.dlog.Logf("getLink on %v of %s with mask %d", base, tag, statusMask)
	b := base.String()
	err = dht.db.View(func(tx *buntdb.Tx) error {
		_, err := _get(tx, b, StatusLive+StatusModified) //only get links on live and modified bases
		if err != nil {
			return err
		}

		if statusMask == StatusDefault {
			statusMask = StatusLive
		}

		results = make([]TaggedHash, 0)
		err = tx.Ascend("link", func(key, value string) bool {
			x := strings.Split(key, ":")

			if string(x[1]) == b && string(x[3]) == tag {
				var status int
				status, err = strconv.Atoi(value)
				if err == nil && (status&statusMask) > 0 {
					results = append(results, TaggedHash{H: string(x[2])})
				}
			}

			return true
		})

		if len(results) == 0 {
			err = fmt.Errorf("No links for %s", tag)
		}
		return err
	})
	return
}

func (dht *DHT) Send(key Hash, msgType MsgType, body interface{}) (response interface{}, err error) {
	n, err := dht.FindNodeForHash(key)
	if err != nil {
		return
	}
	response, err = dht.send(n.HashAddr, msgType, body)
	return
}

// Send sends a message to the node
func (dht *DHT) send(to peer.ID, t MsgType, body interface{}) (response interface{}, err error) {
	return dht.h.Send(ActionProtocol, to, t, body)
}

// FindNodeForHash gets the nearest node to the neighborhood of the hash
func (dht *DHT) FindNodeForHash(key Hash) (n *Node, err error) {

	// for now, the node it returns it self!
	pid := dht.h.nodeID

	var node Node
	node.HashAddr = pid

	n = &node

	return
}

// HandleChangeReqs waits on a chanel for messages to handle
/*func (dht *DHT) HandleChangeReqs() (err error) {
	for {
		dht.dlog.Log("HandleChangeReq: waiting for request")
		m, ok := <-dht.puts
		if !ok {
			dht.dlog.Log("HandleChangeReq: channel closed, breaking")
			break
		}

		err = dht.handleChangeReq(&m)
		if err != nil {
			dht.dlog.Logf("HandleChangeReq: got err: %v", err)
		}
	}
	return nil
}
*/

// Start initiates listening for DHT & Gossip protocol messages on the node
func (dht *DHT) Start() (err error) {
	err = dht.h.node.StartProtocol(dht.h, GossipProtocol)
	return
}

// DumpIdx converts message and data of a DHT change request to a string for human consumption
func (dht *DHT) DumpIdx(idx int) (str string, err error) {
	var msg Message
	msg, err = dht.GetIdxMessage(idx)
	if err != nil {
		return
	}
	f, _ := msg.Fingerprint()
	str = fmt.Sprintf("MSG (fingerprint %v):\n   %v\n", f, msg)
	switch msg.Type {
	case PUT_REQUEST:
		key := msg.Body.(PutReq).H
		entry, entryType, _, _, e := dht.get(key, StatusDefault, GetMaskAll)
		if e != nil {
			err = fmt.Errorf("couldn't get %v err:%v ", key, e)
			return
		} else {
			str += fmt.Sprintf("DATA: type:%s entry: %v\n", entryType, entry)
		}
	}
	return
}
