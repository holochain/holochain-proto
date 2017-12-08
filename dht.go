// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// DHT implements the distributed hash table

package holochain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	"github.com/tidwall/buntdb"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Holds the dht configuration options
type DHTConfig struct {
	// HashType : (string) Identifies hash type to be used for this application. Should be from the list of hash types from the multihash library
	HashType string

	// NeighborhoodSize(integer) Establishes minimum online redundancy targets for data, and size of peer sets for sync gossip. A neighborhood size of ZERO means no sharding (every node syncs all data with every other node). ONE means you are running this as a centralized application and gossip is turned OFF. For most applications we recommend neighborhoods no smaller than 8 for nearness or 32 for hashmask sharding.
	NeighborhoodSize int

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
	h          *Holochain // pointer to the holochain this DHT is part of
	db         *buntdb.DB
	retryQueue chan *retry
	gossipPuts chan Put
	glog       *Logger // the gossip logger
	dlog       *Logger // the dht logger
	gchan      chan gossipWithReq
	config     *DHTConfig
	glk        sync.RWMutex
	//	sources      map[peer.ID]bool
	//	fingerprints map[string]bool
}

type retry struct {
	msg     Message
	retries int
}

const (
	MaxRetries = 10
)

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
	Entry      GobEntry
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
	Links Hash // hash of the source entry making the link, i.e. the req provenance
}

// LinkQuery holds a getLinks query
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

// GetLinksOptions options to holochain level GetLinks functions
type GetLinksOptions struct {
	Load       bool // indicates whether GetLinks should retrieve the entries of all links
	StatusMask int  // mask of which status of links to return
}

// TaggedHash holds associated entries for the LinkQueryResponse
type TaggedHash struct {
	H         string // the hash of the link; gets filled by dht base node when answering get link request
	E         string // the value of link, gets filled if options set Load to true
	EntryType string // the entry type of the link, gets filled if options set Load to true
	T         string // the tag of the link, gets filled only if a tag wasn't specified and all tags are being returns
	Source    string // the statuses on the link, gets filled if options set Load to true
}

// LinkQueryResp holds response to getLinks query
type LinkQueryResp struct {
	Links []TaggedHash
}

type ListAddReq struct {
	ListType    string
	Peers       []string
	WarrantType int
	Warrant     []byte
}

// LinkEvent represents the value stored in buntDB associated with a
// link key for one source having stored one LinkingEntry
// (The Link struct defined in entry.go is encoded in the key used for buntDB)
type LinkEvent struct {
	Status     int
	Source     string
	LinksEntry string
}

var ErrLinkNotFound = errors.New("link not found")
var ErrPutLinkOverDeleted = errors.New("putlink over deleted link")
var ErrHashDeleted = errors.New("hash deleted")
var ErrHashModified = errors.New("hash modified")
var ErrHashRejected = errors.New("hash rejected")
var ErrEntryTypeMismatch = errors.New("entry type mismatch")

var KValue int = 10
var AlphaValue int = 3

const (
	GossipWithQueueSize = 10
	GossipPutQueueSize  = 1000
)

// NewDHT creates a new DHT structure
func NewDHT(h *Holochain) *DHT {
	dht := DHT{
		h:      h,
		glog:   &h.Config.Loggers.Gossip,
		dlog:   &h.Config.Loggers.DHT,
		config: &h.Nucleus().DNA().DHTConfig,
	}
	db, err := buntdb.Open(filepath.Join(h.DBPath(), DHTStoreFileName))
	if err != nil {
		panic(err)
	}
	db.CreateIndex("link", "link:*", buntdb.IndexString)
	db.CreateIndex("idx", "idx:*", buntdb.IndexInt)
	db.CreateIndex("peer", "peer:*", buntdb.IndexString)
	db.CreateIndex("list", "list:*", buntdb.IndexString)
	db.CreateIndex("entry", "entry:*", buntdb.IndexString)

	dht.db = db
	dht.retryQueue = make(chan *retry, 100)

	//	dht.sources = make(map[peer.ID]bool)
	//	dht.fingerprints = make(map[string]bool)
	dht.gchan = make(chan gossipWithReq, GossipWithQueueSize)
	dht.gossipPuts = make(chan Put, GossipPutQueueSize)

	return &dht
}

// putKey implements the special case for adding the KeyEntry system type to the DHT
// note that the Contents of this key are the same as the contents of the agent entry on the
// chain.  The keyEntry is a virtual entry that's NOT actually on the chain
func (dht *DHT) putKey(agent Agent) (err error) {
	var nodeID peer.ID
	var nodeIDStr string
	nodeID, nodeIDStr, err = agent.NodeID()
	if err != nil {
		return
	}
	keyHash, err := NewHash(nodeIDStr)
	if err != nil {
		return
	}

	var pubKey []byte
	pubKey, err = ic.MarshalPublicKey(agent.PubKey())
	if err != nil {
		return
	}
	if err = dht.put(dht.h.node.NewMessage(PUT_REQUEST, PutReq{H: keyHash}), KeyEntryType, keyHash, nodeID, pubKey, StatusLive); err != nil {
		return
	}
	return
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
	err = dht.putKey(dht.h.agent) // first time so revocation is empty

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
			err = _link(tx, k, link, SysTagReplacedBy, m.From, StatusLive, newkey)
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

// _link is a low level routine to add a link, also used by delLink
// this ensure monotonic recording of linking attempts
func _link(tx *buntdb.Tx, base string, link string, tag string, src peer.ID, status int, linkingEntryHash Hash) (err error) {
	key := "link:" + base + ":" + link + ":" + tag
	var val string
	val, err = tx.Get(key)
	source := peer.IDB58Encode(src)
	lehStr := linkingEntryHash.String()
	var records []LinkEvent
	if err == nil {
		// load the previous value so we can append to it.
		json.Unmarshal([]byte(val), &records)

		// TODO: if the link exists, then load the statuses and see
		// what we should do about this situation
		/*
			// search for the source and linking entry in the status
			for _, s := range records {
				if s.Source == source && s.LinksEntry == lehStr {
					if status == StatusLive && s.Status != status {
						err = ErrPutLinkOverDeleted
						return
					}
					// return silently because this is just a duplicate putLink
					break
				}
			} // fall through and add this linking event.
		*/

	} else if err == buntdb.ErrNotFound {
		// when deleting the key must exist
		if status == StatusDeleted {
			err = ErrLinkNotFound
			return
		}
		err = nil
	} else {
		return
	}
	records = append(records, LinkEvent{status, source, lehStr})
	var b []byte
	b, err = json.Marshal(records)
	if err != nil {
		return
	}
	_, _, err = tx.Set(key, string(b), nil)
	if err != nil {
		return
	}
	return
}

func (dht *DHT) link(m *Message, base string, link string, tag string, status int) (err error) {
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		_, err := _get(tx, base, StatusLive)
		if err != nil {
			return err
		}
		err = _link(tx, base, link, tag, m.From, status, m.Body.(LinkReq).Links)
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

// putLink associates a link with a stored hash
// N.B. this function assumes that the data associated has been properly retrieved
// and validated from the cource chain
func (dht *DHT) putLink(m *Message, base string, link string, tag string) (err error) {
	dht.dlog.Logf("putLink on %v link %v as %s", base, link, tag)
	err = dht.link(m, base, link, tag, StatusLive)
	return
}

// delLink removes a link and tag associated with a stored hash
// N.B. this function assumes that the action has been properly validated
func (dht *DHT) delLink(m *Message, base string, link string, tag string) (err error) {
	dht.dlog.Logf("delLink on %v link %v as %s", base, link, tag)
	err = dht.link(m, base, link, tag, StatusDeleted)
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

// getLinks retrieves meta value associated with a base
func (dht *DHT) getLinks(base Hash, tag string, statusMask int) (results []TaggedHash, err error) {
	dht.dlog.Logf("getLinks on %v of %s with mask %d", base, tag, statusMask)
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
			t := string(x[3])
			if string(x[1]) == b && (tag == "" || tag == t) {
				var records []LinkEvent
				json.Unmarshal([]byte(value), &records)
				l := len(records)
				//TODO: this is totally bogus currently simply
				// looking at the last item we ever got
				if l > 0 {
					entry := records[l-1]
					if err == nil && (entry.Status&statusMask) > 0 {
						th := TaggedHash{H: string(x[2]), Source: entry.Source}
						if tag == "" {
							th.T = t
						}
						results = append(results, th)
					}
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

// Change sends DHT change messages to the closest peers to the hash in question
func (dht *DHT) Change(key Hash, msgType MsgType, body interface{}) (err error) {
	dht.h.Debugf("Starting %v Change for %v with body %v", msgType, key, body)

	msg := dht.h.node.NewMessage(msgType, body)
	// change in our local DHT as well as
	_, err = dht.send(nil, dht.h.nodeID, msg)

	if err != nil {
		dht.dlog.Logf("DHT send of %v to self failed with error: %s", msgType, err)
		err = nil
	}
	node := dht.h.node

	pchan, err := node.GetClosestPeers(node.ctx, key)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for p := range pchan {
		wg.Add(1)
		go func(p peer.ID) {
			ctx, cancel := context.WithCancel(node.ctx)
			defer cancel()
			defer wg.Done()

			_, err := dht.send(ctx, p, msg)
			if err != nil {
				dht.dlog.Logf("DHT send of %v failed to peer %v with error: %s", msgType, p, err)
			}
		}(p)
	}
	wg.Wait()
	return
}

// Query sends DHT query messages recursively to peers until one is able to respond.
func (dht *DHT) Query(key Hash, msgType MsgType, body interface{}) (response interface{}, err error) {
	dht.h.Debugf("Starting %v Query for %v with body %v", msgType, key, body)

	msg := dht.h.node.NewMessage(msgType, body)
	// try locally first
	response, err = dht.send(nil, dht.h.nodeID, msg)
	if err == nil {
		// if we actually got a response (not a closer peers list) then return it
		_, notok := response.(CloserPeersResp)
		if !notok {
			return
		}
	} else {
		if err != ErrHashNotFound {
			return
		}
		err = nil
	}

	// get closest peers in the routing table
	rtp := dht.h.node.routingTable.NearestPeers(key, AlphaValue)
	dht.h.Debugf("peers in rt: %d %s", len(rtp), rtp)
	if len(rtp) == 0 {
		Info("DHT Query with no peers in routing table!")
		return nil, ErrHashNotFound
	}

	// setup the Query
	query := dht.h.node.newQuery(key, func(ctx context.Context, to peer.ID) (*dhtQueryResult, error) {

		response, err := dht.send(ctx, to, msg)
		if err != nil {
			dht.h.Debugf("Query failed: %v", err)
			return nil, err
		}

		res := &dhtQueryResult{}

		switch t := response.(type) {
		case LinkQueryResp:
			dht.h.Debugf("Query successful with: %v", response)
			res.success = true
			res.response = &t
		case GetResp:
			dht.h.Debugf("Query successful with: %v", response)
			res.success = true
			res.response = response
		case CloserPeersResp:
			res.closerPeers = peerInfos2Pis(t.CloserPeers)
		default:
			err = fmt.Errorf("unknown response type %T in query", t)
			return nil, err
		}
		return res, nil
	})

	// run it!
	var result *dhtQueryResult
	result, err = query.Run(dht.h.node.ctx, rtp)
	if err != nil {
		return nil, err
	}
	response = result.response
	return
}

// Send sends a message to the node
func (dht *DHT) send(ctx context.Context, to peer.ID, msg *Message) (response interface{}, err error) {
	if ctx == nil {
		ctx = dht.h.node.ctx
	}
	return dht.h.Send(ctx, ActionProtocol, to, msg, 0)
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
	if err = dht.h.node.StartProtocol(dht.h, GossipProtocol); err != nil {
		return
	}
	err = dht.h.node.StartProtocol(dht.h, KademliaProtocol)
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

func statusValueToString(val string) string {
	//TODO
	return val
}

// String converts a DHT into a human readable string
func (dht *DHT) String() (result string) {
	idx, err := dht.GetIdx()
	if err != nil {
		return err.Error()
	}
	result += fmt.Sprintf("DHT changes: %d\n", idx)
	for i := 1; i <= idx; i++ {
		str, err := dht.DumpIdx(i)
		if err != nil {
			result += fmt.Sprintf("%d Error:%v\n", i, err)
		} else {
			result += fmt.Sprintf("%d\n%v\n", i, str)
		}
	}

	result += fmt.Sprintf("DHT entries:\n")
	err = dht.db.View(func(tx *buntdb.Tx) error {
		err = tx.Ascend("entry", func(key, value string) bool {
			x := strings.Split(key, ":")
			k := string(x[1])
			var status string
			statusVal, err := tx.Get("status:" + k)
			if err != nil {
				status = fmt.Sprintf("<err getting status:%v>", err)
			} else {
				status = statusValueToString(statusVal)
			}

			var sources string
			sources, err = tx.Get("src:" + k)
			if err != nil {
				sources = fmt.Sprintf("<err getting sources:%v>", err)
			}
			var links string
			err = tx.Ascend("link", func(key, value string) bool {
				x := strings.Split(key, ":")
				base := x[1]
				link := x[2]
				tag := x[3]
				if base == k {
					links += fmt.Sprintf("Linked to: %s with tag %s\n", link, tag)
					links += value + "\n"
				}
				return true
			})
			result += fmt.Sprintf("Hash--%s (status %s):\nValue: %s\nSources: %s\n%s\n", k, status, value, sources, links)
			return true
		})
		return nil
	})

	return
}

// Close cleans up the DHT
func (dht *DHT) Close() {
	close(dht.retryQueue)
	dht.retryQueue = nil
	close(dht.gchan)
	dht.gchan = nil
	close(dht.gossipPuts)
	dht.gossipPuts = nil
	dht.db.Close()
	dht.db = nil
}

// Retry starts retry processing
func RetryTask(h *Holochain) {
	dht := h.dht
	if dht != nil && len(dht.retryQueue) > 0 {
		r := <-dht.retryQueue
		if r.retries > 0 {
			resp, err := actionReceiver(dht.h, &r.msg, r.retries-1)
			dht.dlog.Logf("retry %d of %v, response: %d error: %v", r.retries, r.msg, resp, err)
		} else {
			dht.dlog.Logf("max retries for %v, ignoring", r.msg)
		}
	}
}
