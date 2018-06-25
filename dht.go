// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// DHT implements the distributed hash table

package holochain

import (
	"context"
	"errors"
	"fmt"
	"gopkg.in/mgo.v2/bson"
	"path/filepath"
	"sync"

	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
)

type HashType string

// Holds the dht configuration options
type DHTConfig struct {
	// HashType : (string) Identifies hash type to be used for this application. Should be from the list of hash types from the multihash library
	HashType HashType

	//RedundancyFactor(integer) Establishes minimum online redundancy targets for data, and size of peer sets for sync gossip. A redundancy factor ZERO means no sharding (every node syncs all data with every other node). ONE means you are running this as a centralized application and gossip is turned OFF. For most applications we recommend neighborhoods no smaller than 8 for nearness or 32 for hashmask sharding.
	RedundancyFactor int

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

type Channel chan interface{}

// DHT struct holds the data necessary to run the distributed hash table
type DHT struct {
	h           *Holochain // pointer to the holochain this DHT is part of
	ht          HashTable
	retryQueue  chan *retry
	changeQueue Channel
	gossipPuts  Channel
	glog        *Logger // the gossip logger
	dlog        *Logger // the dht logger
	gchan       Channel
	config      *DHTConfig
	glk         sync.RWMutex
	//	sources      map[peer.ID]bool
	//	fingerprints map[string]bool
}

type changeReq struct {
	key Hash
	msg Message
}

type retry struct {
	msg     Message
	retries int
}

const (
	MaxRetries = 10
)

// HoldReq holds the data of a change
type HoldReq struct {
	EntryHash   Hash // hash of the entry responsible for the change
	RelatedHash Hash // hash of the related entry (link=base,del=deleted, mod=modified by)
}

// HoldResp holds the signature and code of how a hold request was treated
type HoldResp struct {
	Code      int
	Signature Signature
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
	Bundle     bool // bool if get should happen from bundle not DHT
}

// GetLinksOptions options to holochain level GetLinks functions
type GetLinksOptions struct {
	Load       bool // indicates whether GetLinks should retrieve the entries of all links
	StatusMask int  // mask of which status of links to return
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

var KValue int = 10
var AlphaValue int = 3

const (
	GossipWithQueueSize = 10
	GossipPutQueueSize  = 1000
)

var ErrNotAcceptedByAnyRemoteNode = errors.New("Change not accepted by any remote node")

// NewDHT creates a new DHT structure
func NewDHT(h *Holochain) *DHT {
	dht := DHT{}
	dht.Open(h)
	return &dht
}

// Open sets up the DHTs data structures and store
func (dht *DHT) Open(options interface{}) (err error) {
	h := options.(*Holochain)
	dht.h = h
	dht.glog = &h.Config.Loggers.Gossip
	dht.dlog = &h.Config.Loggers.DHT
	dht.config = &h.Nucleus().DNA().DHTConfig

	dht.ht = &BuntHT{}
	dht.ht.Open(filepath.Join(h.DBPath(), DHTStoreFileName))
	dht.retryQueue = make(chan *retry, 100)
	dht.changeQueue = make(Channel, 1000)
	//go dht.HandleChangeRequests()

	//	dht.sources = make(map[peer.ID]bool)
	//	dht.fingerprints = make(map[string]bool)
	dht.gchan = make(Channel, GossipWithQueueSize)
	dht.gossipPuts = make(Channel, GossipPutQueueSize)
	return
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
	var pubKey string
	pubKey, err = agent.EncodePubKey()
	if err != nil {
		return
	}
	if err = dht.Put(dht.h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: keyHash}), KeyEntryType, keyHash, nodeID, []byte(pubKey), StatusLive); err != nil {
		return
	}
	return
}

// SetupDHT prepares a DHT for use by putting the genesis entries that are added by GenChain
func (dht *DHT) SetupDHT() (err error) {
	x := ""
	// put the holochain id so it always exists for linking
	dna := dht.h.DNAHash()
	err = dht.Put(nil, DNAEntryType, dna, dht.h.nodeID, []byte(x), StatusLive)
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
	if err = dht.Put(dht.h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: a}), AgentEntryType, a, dht.h.nodeID, b, StatusLive); err != nil {
		return
	}

	return
}

// Put stores a value to the DHT store
// N.B. This call assumes that the value has already been validated
func (dht *DHT) Put(m *Message, entryType string, key Hash, src peer.ID, value []byte, status int) (err error) {
	dht.dlog.Logf("put %v=>%s", key, string(value))
	err = dht.ht.Put(m, entryType, key, src, value, status)
	return
}

// Del moves the given hash to the StatusDeleted status
// N.B. this functions assumes that the validity of this action has been confirmed
func (dht *DHT) Del(m *Message, key Hash) (err error) {
	dht.dlog.Logf("del %v", key)
	err = dht.ht.Del(m, key)
	return
}

// Mod moves the given hash to the StatusModified status
// N.B. this functions assumes that the validity of this action has been confirmed
func (dht *DHT) Mod(m *Message, key Hash, newkey Hash) (err error) {
	dht.dlog.Logf("mod %v", key)
	err = dht.ht.Mod(m, key, newkey)
	return
}

// Exists checks for the existence of the hash in the store
func (dht *DHT) Exists(key Hash, statusMask int) (err error) {
	err = dht.ht.Exists(key, statusMask)
	return
}

// Source returns the source node address of a given hash
func (dht *DHT) Source(key Hash) (id peer.ID, err error) {
	id, err = dht.ht.Source(key)
	return
}

// Get retrieves a value from the DHT store
func (dht *DHT) Get(key Hash, statusMask int, getMask int) (data []byte, entryType string, sources []string, status int, err error) {
	data, entryType, sources, status, err = dht.ht.Get(key, statusMask, getMask)
	return
}

// PutLink associates a link with a stored hash
// N.B. this function assumes that the data associated has been properly retrieved
// and validated from the cource chain
func (dht *DHT) PutLink(m *Message, base string, link string, tag string) (err error) {
	dht.dlog.Logf("putLink on %v link %v as %s", base, link, tag)
	err = dht.ht.PutLink(m, base, link, tag)
	return
}

// DelLink removes a link and tag associated with a stored hash
// N.B. this function assumes that the action has been properly validated
func (dht *DHT) DelLink(m *Message, base string, link string, tag string) (err error) {
	dht.dlog.Logf("delLink on %v link %v as %s", base, link, tag)
	err = dht.ht.DelLink(m, base, link, tag)
	return
}

// GetLinks retrieves meta value associated with a base
func (dht *DHT) GetLinks(base Hash, tag string, statusMask int) (results []TaggedHash, err error) {
	dht.dlog.Logf("getLinks on %v of %s with mask %d", base, tag, statusMask)
	results, err = dht.ht.GetLinks(base, tag, statusMask)
	return
}

// HandleChangeRequests waits on a channel for dht change requests
func (dht *DHT) HandleChangeRequests() (err error) {
	err = dht.handleTillDone("HandleChangeRequests", dht.changeQueue, handleChangeRequests)
	return
}

func handleChangeRequests(dht *DHT, x interface{}) (err error) {
	req := x.(changeReq)
	err = dht.change(req)
	return
}

func (dht *DHT) sendChange(p peer.ID, msg *Message) (held bool, err error) {
	if dht == nil || dht.h.node == nil {
		return
	}
	ctx, cancel := context.WithCancel(dht.h.node.ctx)
	defer cancel()

	resp, err := dht.send(ctx, p, msg)
	if err != nil {
		return
	} else {
		switch t := resp.(type) {
		case HoldResp:
			if t.Code == ReceiptRejected {
				// TODO what else do we do if rejected?
				dht.dlog.Logf("DHT send of %v failed to peer %v was rejected", msg, p)
			}
			held = true
			// TODO check the signature on the receipt
		case CloserPeersResp:
			closerPeers := peerInfos2Pis(t.CloserPeers)
			//	s := fmt.Sprintf("%v says closer to %v are: ", p.Pretty()[2:4], key)

			for _, closer := range closerPeers {
				//		s += fmt.Sprintf("%v ", closer.ID.Pretty()[2:4])
				dht.h.AddPeer(*closer)
			}
			//	fmt.Printf("%s\n", s)

		default:
			err = fmt.Errorf("DHT sendChange of %v to peer %v response(%T) was: %v", msg.Type, p, t, t)
		}
	}
	return
}

func (dht *DHT) change(req changeReq) (err error) {
	key := req.key
	msg := &req.msg
	node := dht.h.node
	pchan, err := node.GetClosestPeers(node.ctx, key)
	if err != nil {
		return err
	}
	var held []peer.ID
	wg := sync.WaitGroup{}
	for p := range pchan {
		if p == node.HashAddr {
			continue
		}
		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			wasHeld, err := dht.sendChange(p, msg)
			if err != nil {
				dht.dlog.Logf("DHT sendChange of %v failed to peer %v with error: %s", msg.Type, p, err)
			} else if wasHeld {
				held = append(held, p)
			}
		}(p)
	}
	wg.Wait()
	if dht.h.Config.EnableWorldModel {
		for _, p := range held {
			err := dht.h.world.SetNodeHolding(p, key)
			if err != nil {
				dht.dlog.Logf("SetNodeHolding for node %v not found in world node", p)
			}
		}
	}
	return
}

// Change sends DHT change messages to the closest peers to the hash in question
func (dht *DHT) Change(key Hash, msgType MsgType, body interface{}) (err error) {
	dht.h.Debugf("Starting %v Change for %v with body %v", msgType, key, body)

	msg := dht.h.node.NewMessage(msgType, body)
	// change in our local DHT
	_, err = dht.send(nil, dht.h.nodeID, msg)
	if err != nil {
		return err
	}
	/*	if err != nil {
		dht.dlog.Logf("DHT send of %v to self failed with error: %s", msgType, err)
		err = nil
	}*/
	dht.changeQueue <- changeReq{msg: *msg, key: key}

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

// String converts a DHT into a human readable string
func (dht *DHT) String() (result string) {
	return dht.ht.String()
}

// JSON converts a DHT into a JSON string representation.
func (dht *DHT) JSON() (result string, err error) {
	result, err = dht.ht.JSON()
	return
}

// Close cleans up the DHT
func (dht *DHT) Close() {
	close(dht.changeQueue)
	dht.changeQueue = nil
	close(dht.retryQueue)
	dht.retryQueue = nil
	close(dht.gchan)
	dht.gchan = nil
	close(dht.gossipPuts)
	dht.gossipPuts = nil
	dht.ht.Close()
}

// GetIdx returns the current index of changes to the HashTable
func (dht *DHT) GetIdx() (idx int, err error) {
	idx, err = dht.ht.GetIdx()
	return
}

// GetIdxMessage returns the messages that causes the change at a given index
func (dht *DHT) GetIdxMessage(idx int) (msg Message, err error) {
	msg, err = dht.ht.GetIdxMessage(idx)
	return
}

// RetryTask checks to see if there are any received puts that need retrying and does one if so
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

// MakeReceiptData converts a message and a code into signable data
func MakeReceiptData(msg *Message, code int) (reciept []byte, err error) {
	var data []byte

	data, err = bson.Marshal(msg)
	if err != nil {
		return
	}

	reciept = append(data, byte(code))

	return
}

// MakeReceipt creates a signature of a message together with the receipt code
func (dht *DHT) MakeReceiptSignature(msg *Message, code int) (sig Signature, err error) {
	var data []byte
	data, err = MakeReceiptData(msg, code)
	if err != nil {
		return
	}
	sig, err = dht.h.Sign(data)
	return
}

// MakeHoldResp creates fill the HoldResp struct with a the holding status and signature
func (dht *DHT) MakeHoldResp(msg *Message, status int) (holdResp *HoldResp, err error) {
	hr := HoldResp{}
	if status == StatusRejected {
		hr.Code = ReceiptRejected
	} else {
		hr.Code = ReceiptOK
	}
	hr.Signature, err = dht.MakeReceiptSignature(msg, hr.Code)
	if err == nil {
		holdResp = &hr
	}
	return
}

func (dht *DHT) Iterate(fn HashTableIterateFn) {
	dht.ht.Iterate(fn)
}
