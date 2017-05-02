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

// constants for the state of the data, they are bit flags
const (
	StatusLive     = 0x01
	StatusRejected = 0x02
	StatusDeleted  = 0x04
	StatusModified = 0x08
	StatusAny      = 0xFF
)

// constants for the stored string status values in buntdb
const (
	StatusLiveVal     = "1"
	StatusRejectedVal = "2"
	StatusDeletedVal  = "4"
	StatusModifiedVal = "8"
	StatusAnyVal      = "255"
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
	err = dht.put(nil, DNAEntryType, dht.h.DNAHash(), dht.h.id, []byte(x), StatusLive)
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
	if err = dht.put(nil, AgentEntryType, a, dht.h.id, b, StatusLive); err != nil {
		return
	}

	// put the KeyEntry so it always exists for linking
	kh, err := NewHash(peer.IDB58Encode(dht.h.id))
	if err != nil {
		return
	}
	if err = dht.put(nil, KeyEntryType, kh, dht.h.id, []byte(dht.h.id), StatusLive); err != nil {
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

// del moves the given hash to the StatusDeleted status
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

		_, _, err = tx.Set("status:"+k, fmt.Sprintf("%d", StatusDeleted), nil)
		if err != nil {
			return err
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
		if statusMask == 0 {
			statusMask = StatusLive
		}
		var status int
		status, err = strconv.Atoi(statusVal)
		if err == nil {
			if (status & statusMask) == 0 {
				err = ErrHashNotFound
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
func (dht *DHT) get(key Hash, statusMask int) (data []byte, entryType string, status int, err error) {
	err = dht.db.View(func(tx *buntdb.Tx) error {
		k := key.String()
		val, err := _get(tx, k, statusMask)
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
		_, err := _get(tx, base, StatusLive)
		if err != nil {
			return err
		}

		key := "link:" + base + ":" + link + ":" + tag
		_, err = tx.Get(key)
		if err == buntdb.ErrNotFound {

			//var index string
			_, err = incIdx(tx, m)
			if err != nil {
				return err
			}

			_, _, err = tx.Set(key, StatusLiveVal, nil)
			if err != nil {
				return err
			}
		} else {
			//TODO what do we do if there's already something there?
			panic("putLink over existing link not implemented")
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
		_, err := _get(tx, b, StatusLive) //only get links on live bases
		if err != nil {
			return err
		}

		if statusMask == 0 {
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
			a := NewPutAction(resp.Type, &resp.Entry, &resp.Header)
			_, err = dht.h.ValidateAction(a, a.entryType, []peer.ID{from})

			var status int
			if err != nil {
				dht.dlog.Logf("Put %v rejected: %v", t.H, err)
				status = StatusRejected
			} else {
				status = StatusLive
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
		_, _, hashStatus, err = dht.get(t.H, StatusAny)
		if err != nil {
			if err == ErrHashNotFound {
				dht.dlog.Logf("don't yet have %s, trying again later", t.H)
				panic("RETRY-DELETE NOT IMPLEMENTED")
				// try the del again later
			}
			return
		}

		if hashStatus == StatusLive {
			var r interface{}
			r, err = dht.h.Send(ValidateProtocol, from, VALIDATE_DEL_REQUEST, ValidateQuery{H: t.H})
			if err != nil {
				return
			}

			switch resp := r.(type) {
			case ValidateDelResponse:
				a := NewDelAction(t.H)
				//@TODO what comes back from Validate Del
				_, err = dht.h.ValidateAction(a, resp.Type, []peer.ID{from})
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
			dht.dlog.Logf("%s isn't StatusLive, can't DEL", t.H)
			// @TODO what happens if the hashStatus is not StatusLive?
		}

	case LinkReq:

		//var baseType string
		//var baseStatus int
		_, _, _, err = dht.get(t.Base, StatusLive)
		// @TODO what happens if the baseStatus is not StatusLive?
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
			a := NewLinkAction(resp.LinkingType, resp.Links)
			a.validationBase = t.Base
			_, err = dht.h.ValidateAction(a, a.entryType, []peer.ID{from})
			//@TODO this is "one bad apple spoils the lot" because the app
			// has no way to tell us not to link certain of the links.
			// we need to extend the return value of the app to be able to
			// have it reject a subset of the links.
			if err != nil {
				// how do we record an invalid linking?
				//@TODO store as REJECTED
			} else {
				base := t.Base.String()
				for _, l := range resp.Links {
					if base == l.Base {
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
func DHTReceiver(h *Holochain, msg *Message) (response interface{}, err error) {
	dht := h.dht
	var a Action
	a, err = h.GetDHTReqAction(msg)
	if err == nil {
		dht.dlog.Logf("DHTReceiver got %s: %v", a.Name(), msg)
		// N.B. DHTReqHandler calls made to an Action whose values are NOT populated
		// the handler's understand this and use the values from the message body
		response, err = a.DHTReqHandler(dht, msg)
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
