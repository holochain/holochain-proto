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

var ErrDHTExpectedGetReqInBody = errors.New("expected get request")
var ErrDHTExpectedPutReqInBody = errors.New("expected put request")
var ErrDHTExpectedMetaReqInBody = errors.New("expected meta request")
var ErrDHTExpectedMetaQueryInBody = errors.New("expected meta query")

// DHT struct holds the data necessary to run the distributed hash table
type DHT struct {
	h         *Holochain // pointer to the holochain this DHT is part of
	db        *buntdb.DB
	puts      chan *Message
	gossiping bool
	glog      Logger // the gossip logger
	dlog      Logger // the dht logger
}

// Meta holds data that can be associated with a hash
// @todo, we should also be storing the meta-data source
type Meta struct {
	H Hash   // hash of meta-data associated
	T string // meta-data type identifier
	V []byte // meta-data
}

// constants for the Put type
const (
	PutNew = iota
	PutUpdate
	PutDelete
	PutUndelete
)

// constants for the state of the meta data
const (
	LIVE = iota
	REJECTED
	DELETED
	UPDATED
)

// PutReq holds the data of a put request
type PutReq struct {
	H Hash
	S int
	D interface{}
}

// GetReq holds the data of a put request
type GetReq struct {
	H Hash
}

// MetaReq holds a putMeta request
type MetaReq struct {
	Base Hash // original data on which to put the meta
	M    Hash // hash of the meta-data
	T    Hash // hash of the tag entry
}

// MetaQuery holds a getMeta query
type MetaQuery struct {
	Base Hash
	T    string
	// order
	// filter, etc
}

// TaggedHash holds associated entries for the MetaQueryResponse
type TaggedHash struct {
	H string
}

// MetaQueryResp holds response to getMeta query
type MetaQueryResp struct {
	Hashes []TaggedHash
}

// NewDHT creates a new DHT structure
func NewDHT(h *Holochain) *DHT {
	dht := DHT{
		h: h,
	}
	db, err := buntdb.Open(h.DBPath() + "/" + DHTStoreFileName)
	if err != nil {
		panic(err)
	}
	db.CreateIndex("meta", "meta:*", buntdb.IndexString)
	db.CreateIndex("idx", "idx:*", buntdb.IndexInt)
	db.CreateIndex("peer", "peer:*", buntdb.IndexString)

	dht.db = db
	dht.puts = make(chan *Message, 10)

	dht.glog = h.config.Loggers.Gossip
	dht.dlog = h.config.Loggers.DHT

	return &dht
}

// SetupDHT prepares a DHT for use by adding the holochain's ID
func (dht *DHT) SetupDHT() (err error) {
	x := ""
	// put the holochain id so it always exists for putmeta
	err = dht.put(nil, DNAEntryType, dht.h.DNAHash(), dht.h.id, []byte(x), LIVE)
	if err != nil {
		return
	}
	// put the AgentEntry so it always exists for putmeta
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
	if err = dht.put(nil, AgentEntryType, a, dht.h.id, b, LIVE); err != nil {
		return
	}

	// put the KeyEntry so it always exists for putmeta
	kh, err := NewHash(peer.IDB58Encode(dht.h.id))
	if err != nil {
		return
	}
	if err = dht.put(nil, KeyEntryType, kh, dht.h.id, []byte(dht.h.id), LIVE); err != nil {
		return
	}

	return
}

// put stores a value to the DHT store
// N.B. This call assumes that the value has already been validated
func (dht *DHT) put(m *Message, entryType string, key Hash, src peer.ID, value []byte, status int) (err error) {
	k := key.String()
	dht.dlog.Logf("put %v=>%s", key, string(value))
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

// exists checks for the existence of the hash in the store
func (dht *DHT) exists(key Hash) (err error) {
	err = dht.db.View(func(tx *buntdb.Tx) error {
		_, err := tx.Get("entry:" + key.String())
		if err == buntdb.ErrNotFound {
			err = ErrHashNotFound
		}
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
func (dht *DHT) get(key Hash) (data []byte, entryType string, status int, err error) {
	err = dht.db.View(func(tx *buntdb.Tx) error {
		k := key.String()
		val, err := tx.Get("entry:" + k)
		if err != nil {
			if err == buntdb.ErrNotFound {
				err = ErrHashNotFound
			}
			return err
		}
		entryType, err = tx.Get("type:" + k)
		if err != nil {
			return err
		}
		if err == nil {
			data = []byte(val)
			val, err = tx.Get("status:" + k)
			status, err = strconv.Atoi(val)
		}
		return err
	})
	return
}

// putMeta associates a value with a stored hash
// N.B. this function assumes that the data associated has been properly retrieved
// and validated from the cource chain
func (dht *DHT) putMeta(m *Message, base Hash, metaKey Hash, tag string) (err error) {
	dht.dlog.Logf("putmeta on %v %v as %s", base, metaKey, tag)
	b := base.String()
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Get("entry:" + b)
		if err == buntdb.ErrNotFound {
			return ErrHashNotFound
		}
		var index string
		index, err = incIdx(tx, m)
		if err != nil {
			return err
		}

		x := "meta:" + index + ":" + b + ":" + metaKey.String()
		_, _, err = tx.Set(x, tag, nil)
		if err != nil {
			return err
		}

		return nil
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

// getMeta retrieves meta value associated with a base
func (dht *DHT) getMeta(base Hash, tag string) (results []TaggedHash, err error) {
	b := base.String()
	err = dht.db.View(func(tx *buntdb.Tx) error {
		_, err := tx.Get("entry:" + b)
		if err == buntdb.ErrNotFound {
			return ErrHashNotFound
		}
		results = make([]TaggedHash, 0)
		err = tx.Ascend("meta", func(key, value string) bool {
			x := strings.Split(key, ":")

			if string(x[2]) == b && value == tag {
				results = append(results, TaggedHash{H: string(x[3])})
			}
			return true
		})
		if len(results) == 0 {
			err = fmt.Errorf("No values for %s", tag)
		}
		return err
	})
	return
}

// SendPut initiates publishing a particular Hash to the DHT.
// This command only sends the hash, because the expectation is that DHT nodes will start to
// communicate back to Source node (the node that makes this call) to get the data for validation
func (dht *DHT) SendPut(key Hash) (err error) {
	n, err := dht.FindNodeForHash(key)
	if err != nil {
		return
	}
	_, err = dht.send(n.HashAddr, PUT_REQUEST, PutReq{H: key})
	return
}

// SendGet initiates retrieving a value from the DHT
func (dht *DHT) SendGet(key Hash) (response interface{}, err error) {
	n, err := dht.FindNodeForHash(key)
	if err != nil {
		return
	}
	response, err = dht.send(n.HashAddr, GET_REQUEST, GetReq{H: key})
	return
}

// SendPutMeta initiates associating Meta data with particular Hash on the DHT.
// This command assumes that the data has been committed to your local chain, and the hash of that
// data is what get's sent in the MetaReq
func (dht *DHT) SendPutMeta(req MetaReq) (err error) {
	n, err := dht.FindNodeForHash(req.Base)
	if err != nil {
		return
	}
	_, err = dht.send(n.HashAddr, PUTMETA_REQUEST, req)
	return
}

// SendGetMeta initiates retrieving meta data from the DHT
func (dht *DHT) SendGetMeta(query MetaQuery) (response interface{}, err error) {
	n, err := dht.FindNodeForHash(query.Base)
	if err != nil {
		return
	}
	response, err = dht.send(n.HashAddr, GETMETA_REQUEST, query)
	return
}

// Send sends a message to the node
func (dht *DHT) send(to peer.ID, t MsgType, body interface{}) (response interface{}, err error) {
	return dht.h.Send(DHTProtocol, to, t, body)
}

// FindNodeForHash gets the nearest node to the neighborhood of the hash
func (dht *DHT) FindNodeForHash(key Hash) (n *Node, err error) {

	// for now, the node it returns is self!
	pid, err := peer.IDFromPrivateKey(dht.h.Agent().PrivKey())
	if err != nil {
		return
	}
	var node Node
	node.HashAddr = pid

	n = &node

	return
}

// HandlePutReqs waits on a chanel for messages to handle
func (dht *DHT) HandlePutReqs() (err error) {
	for {
		dht.dlog.Log("HandlePutReq: waiting for put request")
		m, ok := <-dht.puts
		if !ok {
			break
		}
		err = dht.handlePutReq(m)
		if err != nil {
			dht.dlog.Logf("HandlePutReq: got err: %v", err)
		}
	}
	return nil
}

func (dht *DHT) handlePutReq(m *Message) (err error) {
	from := m.From
	switch t := m.Body.(type) {
	case PutReq:
		dht.dlog.Logf("handling put: %v", m)
		var r interface{}
		r, err = dht.h.Send(ValidateProtocol, from, VALIDATE_REQUEST, ValidateQuery{H: t.H})
		if err != nil {
			return
		}
		switch resp := r.(type) {
		case *ValidateResponse:
			err = dht.h.ValidatePut(resp.Type, resp.Entry, &resp.Header, []peer.ID{from})
			if err != nil {
				//@todo store as INVALID
			} else {
				entry := resp.Entry
				b, err := entry.Marshal()
				if err == nil {
					err = dht.put(m, resp.Type, t.H, from, b, LIVE)
				}
			}
		default:
			err = errors.New("expected ValidateResponse from validator")

		}
	case MetaReq:
		dht.dlog.Logf("handling putmeta: %v", m)

		var baseType string
		//var baseStatus int
		_, baseType, _, err = dht.get(t.Base)
		// @TODO what happens if the baseStatus is not LIVE?
		if err != nil {
			if err == ErrHashNotFound {
				dht.dlog.Logf("don't yet have %s, trying again later", t.Base)
				panic("RETRY-PUTMETA NOT IMPLEMENTED")
				// try the put again later
			}
			return
		}

		var r interface{}
		r, err = dht.h.Send(ValidateProtocol, from, VALIDATEMETA_REQUEST, ValidateQuery{H: t.T})
		if err != nil {
			return
		}
		switch resp := r.(type) {
		case *ValidateMetaResponse:
			err = dht.h.ValidatePutMeta(baseType, t.Base, resp.Type, t.M, resp.Tag, []peer.ID{from})
			if err != nil {
				//@todo store as INVALID
			} else {
				err = dht.putMeta(m, t.Base, t.M, resp.Tag)
			}
		default:
			err = errors.New("expected ValidateMetaResponse from validator")
		}
	default:
		err = errors.New("unexpected body type in handlePutReq")
	}
	return
}

// DHTReceiver handles messages on the dht protocol
func DHTReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	dht := h.dht
	switch m.Type {
	case PUT_REQUEST:
		dht.dlog.Logf("DHTReceiver got PUT_REQUEST: %v", m)
		switch m.Body.(type) {
		case PutReq:
			h.dht.puts <- m
			response = "queued"
		default:
			err = ErrDHTExpectedPutReqInBody
		}
		return
	case GET_REQUEST:
		dht.dlog.Logf("DHTReceiver got GET_REQUEST: %v", m)
		switch t := m.Body.(type) {
		case GetReq:
			var b []byte
			b, _, _, err = h.dht.get(t.H)
			if err == nil {
				var e GobEntry
				err = e.Unmarshal(b)
				if err == nil {
					response = &e
				}
			}

		default:
			err = ErrDHTExpectedGetReqInBody
		}
		return
	case PUTMETA_REQUEST:
		dht.dlog.Logf("DHTReceiver got PUTMETA_REQUEST: %v", m)
		switch t := m.Body.(type) {
		case MetaReq:
			err = h.dht.exists(t.Base)
			if err == nil {
				h.dht.puts <- m
				response = "queued"
			} else {
				dht.dlog.Logf("DHTReceiver key %v doesn't exist, ignoring", t.Base)
			}

		default:
			err = ErrDHTExpectedMetaReqInBody
		}
	case GETMETA_REQUEST:
		dht.dlog.Logf("DHTReceiver got GETMETA_REQUEST: %v", m)
		switch t := m.Body.(type) {
		case MetaQuery:
			var r MetaQueryResp
			r.Hashes, err = h.dht.getMeta(t.Base, t.T)
			response = r
		default:
			err = ErrDHTExpectedMetaQueryInBody
		}

	default:
		err = fmt.Errorf("message type %d not in holochain-dht protocol", int(m.Type))
	}
	return
}

// StartDHT initiates listening for DHT protocol messages on the node
func (dht *DHT) StartDHT() (err error) {
	if err = dht.h.node.StartProtocol(dht.h, DHTProtocol); err != nil {
		return
	}
	dht.h.node.StartProtocol(dht.h, GossipProtocol)
	return
}
