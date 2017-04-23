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
var ErrDHTExpectedDelReqInBody = errors.New("expected del request")
var ErrDHTExpectedLinkReqInBody = errors.New("expected link request")
var ErrDHTExpectedLinkQueryInBody = errors.New("expected link query")

// DHT struct holds the data necessary to run the distributed hash table
type DHT struct {
	h         *Holochain // pointer to the holochain this DHT is part of
	db        *buntdb.DB
	puts      chan Message
	gossiping bool
	glog      Logger // the gossip logger
	dlog      Logger // the dht logger
}

// Meta holds data that can be associated with a hash
// @todo, we should also be storing the meta-data source
type Meta struct {
	H Hash   // hash of link-data associated
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

// constants for the state of the  data
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

// GetReq holds the data of a get request
type GetReq struct {
	H Hash
}

// DelReq holds the data of a del request
type DelReq struct {
	H Hash
}

// LinkReq holds a link request
type LinkReq struct {
	Base  Hash // data on which to attach the links
	Links Hash // hash of the links entry
}

// LinkQuery holds a getLink query
type LinkQuery struct {
	Base Hash
	T    string
	// order
	// filter, etc
}

// GetLinkOptions options to holochain level GetLink functions
type GetLinkOptions struct {
	Load bool // indicates whether GetLink should retrieve the entries of all links
}

// TaggedHash holds associated entries for the LinkQueryResponse
type TaggedHash struct {
	H string // the hash of the link; gets filled by dht base node when answering get link request
	E string // the value of link, get's filled by caller if getlink function set Load to true
}

// LinkQueryResp holds response to getLink query
type LinkQueryResp struct {
	Links []TaggedHash
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
	db.CreateIndex("link", "link:*", buntdb.IndexString)
	db.CreateIndex("idx", "idx:*", buntdb.IndexInt)
	db.CreateIndex("peer", "peer:*", buntdb.IndexString)

	dht.db = db
	dht.puts = make(chan Message, 10)

	dht.glog = h.config.Loggers.Gossip
	dht.dlog = h.config.Loggers.DHT

	return &dht
}

// SetupDHT prepares a DHT for use by adding the holochain's ID
func (dht *DHT) SetupDHT() (err error) {
	x := ""
	// put the holochain id so it always exists for linking
	err = dht.put(nil, DNAEntryType, dht.h.DNAHash(), dht.h.id, []byte(x), LIVE)
	if err != nil {
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
	if err = dht.put(nil, AgentEntryType, a, dht.h.id, b, LIVE); err != nil {
		return
	}

	// put the KeyEntry so it always exists for linking
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

// del moves the given hash to the DELETED status
// N.B. this functions assumes that the validity of this action has been confirmed
func (dht *DHT) del(m *Message, key Hash) (err error) {
	k := key.String()
	dht.dlog.Logf("delete %s", k)
	err = dht.db.Update(func(tx *buntdb.Tx) error {

		_, err := tx.Get("entry:" + k)
		if err != nil {
			if err == buntdb.ErrNotFound {
				err = ErrHashNotFound
			}
			return err
		}

		_, err = incIdx(tx, m)
		if err != nil {
			return err
		}

		_, _, err = tx.Set("status:"+k, fmt.Sprintf("%d", DELETED), nil)
		if err != nil {
			return err
		}
		return err
	})

	return
}

func _get(tx *buntdb.Tx, k string) (string, error) {
	val, err := tx.Get("entry:" + k)
	if err == buntdb.ErrNotFound {
		err = ErrHashNotFound
	}
	return val, err
}

// exists checks for the existence of the hash in the store
func (dht *DHT) exists(key Hash) (err error) {
	err = dht.db.View(func(tx *buntdb.Tx) error {
		_, err := _get(tx, key.String())
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
		val, err := _get(tx, k)
		if err != nil {
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

// putLink associates a link with a stored hash
// N.B. this function assumes that the data associated has been properly retrieved
// and validated from the cource chain
func (dht *DHT) putLink(m *Message, base string, link string, tag string) (err error) {
	dht.dlog.Logf("putLink on %v link %v as %s", base, link, tag)
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		_, err := _get(tx, base)
		if err != nil {
			return err
		}

		var index string
		index, err = incIdx(tx, m)
		if err != nil {
			return err
		}

		x := "link:" + index + ":" + base + ":" + link
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

// getLink retrieves meta value associated with a base
func (dht *DHT) getLink(base Hash, tag string) (results []TaggedHash, err error) {
	dht.dlog.Logf("getLink on %v of %s", base, tag)
	b := base.String()
	err = dht.db.View(func(tx *buntdb.Tx) error {
		_, err := _get(tx, b)
		if err != nil {
			return err
		}

		results = make([]TaggedHash, 0)
		err = tx.Ascend("link", func(key, value string) bool {
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

// SendLink initiates associating links with particular Hash on the DHT.
func (dht *DHT) SendLink(req LinkReq) (err error) {
	n, err := dht.FindNodeForHash(req.Base)
	if err != nil {
		return
	}
	_, err = dht.send(n.HashAddr, LINK_REQUEST, req)
	return
}

// SendGetLink initiates retrieving links from the DHT
func (dht *DHT) SendGetLink(query LinkQuery) (response interface{}, err error) {
	n, err := dht.FindNodeForHash(query.Base)
	if err != nil {
		return
	}
	response, err = dht.send(n.HashAddr, GETLINK_REQUEST, query)
	return
}

// SendDel initiates setting a hash's status on the DHT
func (dht *DHT) SendDel(key Hash) (err error) {
	n, err := dht.FindNodeForHash(key)
	if err != nil {
		return
	}
	_, err = dht.send(n.HashAddr, DEL_REQUEST, DelReq{H: key})
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

// HandleChangeReqs waits on a chanel for messages to handle
func (dht *DHT) HandleChangeReqs() (err error) {
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

func (dht *DHT) handleChangeReq(m *Message) (err error) {
	switch t := m.Body.(type) {
	default:
		dht.dlog.Logf("handling %T: %v", t, m)

	}
	from := m.From
	switch t := m.Body.(type) {
	case PutReq:
		var r interface{}
		r, err = dht.h.Send(ValidateProtocol, from, VALIDATE_PUT_REQUEST, ValidateQuery{H: t.H})
		if err != nil {
			return
		}
		switch resp := r.(type) {
		case ValidateResponse:
			err = dht.h.ValidatePut(resp.Type, &resp.Entry, &resp.Header, []peer.ID{from})
			var status int
			if err != nil {
				status = REJECTED
			} else {
				status = LIVE
			}
			entry := resp.Entry
			var b []byte
			b, err = entry.Marshal()
			if err == nil {
				err = dht.put(m, resp.Type, t.H, from, b, status)
			}

		default:
			err = fmt.Errorf("expected ValidateResponse from validator got %T", r)
		}
	case DelReq:
		//var hashType string
		var hashStatus int
		_, _, hashStatus, err = dht.get(t.H)
		if err != nil {
			if err == ErrHashNotFound {
				dht.dlog.Logf("don't yet have %s, trying again later", t.H)
				panic("RETRY-DELETE NOT IMPLEMENTED")
				// try the del again later
			}
			return
		}

		if hashStatus == LIVE {
			var r interface{}
			r, err = dht.h.Send(ValidateProtocol, from, VALIDATE_DEL_REQUEST, ValidateQuery{H: t.H})
			if err != nil {
				return
			}

			switch resp := r.(type) {
			case ValidateDelResponse:
				//@TODO what comes back from Validate Del
				err = dht.h.ValidateDel(resp.Type, t.H.String(), []peer.ID{from})
				if err != nil {
					// how do we record an invalid DEL?
					//@TODO store as REJECTED
				} else {
					err = dht.del(m, t.H)
				}

			default:
				err = fmt.Errorf("expected ValidateDelResponse from validator got %T", resp)
			}

		} else {
			dht.dlog.Logf("%s isn't LIVE, can't DEL", t.H)
			// @TODO what happens if the hashStatus is not LIVE?
		}

	case LinkReq:

		//var baseType string
		//var baseStatus int
		_, _, _, err = dht.get(t.Base)
		// @TODO what happens if the baseStatus is not LIVE?
		if err != nil {
			if err == ErrHashNotFound {
				dht.dlog.Logf("don't yet have %s, trying again later", t.Base)
				panic("RETRY-LINK NOT IMPLEMENTED")
				// try the put again later
			}
			return
		}

		var r interface{}
		r, err = dht.h.Send(ValidateProtocol, from, VALIDATE_LINK_REQUEST, ValidateQuery{H: t.Links})
		if err != nil {
			return
		}
		switch resp := r.(type) {
		case ValidateLinkResponse:
			base := t.Base.String()
			for _, l := range resp.Links {
				if base == l.Base {
					err = dht.h.ValidateLink(resp.LinkingType, base, l.Link, l.Tag, []peer.ID{from})
					if err != nil {
						// how do we record an invalid link?
						//@TODO store as REJECTED
					} else {
						err = dht.putLink(m, base, l.Link, l.Tag)
					}
				}
			}
		default:
			err = fmt.Errorf("expected ValidateLinkResponse from validator got %T", r)
		}
	default:
		err = fmt.Errorf("unexpected body type %T in handleChangeReq", t)
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
			//h.dht.puts <- *m  TODO add back in queueing
			dht.handleChangeReq(m)

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
	case DEL_REQUEST:
		dht.dlog.Logf("DHTReceiver got DEL_REQUEST: %v", m)
		switch m.Body.(type) {
		case DelReq:
			//h.dht.puts <- *m  TODO add back in queueing
			dht.handleChangeReq(m)

			response = "queued"
		default:
			err = ErrDHTExpectedDelReqInBody
		}
		return
	case LINK_REQUEST:
		dht.dlog.Logf("DHTReceiver got LINKS_REQUEST: %v", m)
		switch t := m.Body.(type) {
		case LinkReq:
			err = h.dht.exists(t.Base)
			if err == nil {
				//h.dht.puts <- *m  TODO add back in queueing
				dht.handleChangeReq(m)

				response = "queued"
			} else {
				dht.dlog.Logf("DHTReceiver key %v doesn't exist, ignoring", t.Base)
			}

		default:
			err = ErrDHTExpectedLinkReqInBody
		}

	case GETLINK_REQUEST:
		dht.dlog.Logf("DHTReceiver got GETLINK_REQUEST: %v", m)
		switch t := m.Body.(type) {
		case LinkQuery:
			var r LinkQueryResp
			r.Links, err = h.dht.getLink(t.Base, t.T)
			response = &r
		default:
			err = ErrDHTExpectedLinkQueryInBody
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
