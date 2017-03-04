// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// DHT implements the distributed hash table

package holochain

import (
	"errors"
	"fmt"
	q "github.com/golang-collections/go-datastructures/queue"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/tidwall/buntdb"
	"strings"
)

var ErrDHTExpectedHashInBody error = errors.New("expected hash")
var ErrDHTExpectedMetaInBody error = errors.New("expected meta struct")

// DHT struct holds the data necessary to run the distributed hash table
type DHT struct {
	h     *Holochain // pointer to the holochain this DHT is part of
	Queue q.Queue    // a queue for incoming puts
	db    *buntdb.DB
}

// Meta holds data that can be associated with a hash
// @todo, we should also be storing the meta-data source
type Meta struct {
	H Hash   // hash of meta-data associated
	T string // meta-data type identifier
	V []byte // meta-data
}

// Meta holds a putMeta request
type MetaReq struct {
	O Hash   // original data on which to put the meta
	M Hash   // hash of the meta-data
	T string // meta type
}

// MetaQuery holds a getMeta query
type MetaQuery struct {
	H Hash
	T string
	// order
	// filter, etc
}

// NewDHT creates a new DHT structure
func NewDHT(h *Holochain) *DHT {
	dht := DHT{
		h: h,
	}
	db, err := buntdb.Open(h.path + "/dht.db")
	if err != nil {
		panic(err)
	}
	db.CreateIndex("meta", "meta:*", buntdb.IndexString)

	dht.db = db
	return &dht
}

// put stores a value to the DHT store
// N.B. This call assumes that the value has already been validated
func (dht *DHT) put(key Hash, src peer.ID, value []byte) (err error) {
	k := key.String()
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("entry:"+k, string(value), nil)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("src:"+k, peer.IDB58Encode(src), nil)
		return err
	})
	//	dht.store[k] = value
	//	dht.sources[k] = src
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
func (dht *DHT) get(key Hash) (data []byte, err error) {
	err = dht.db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("entry:" + key.String())
		if err == buntdb.ErrNotFound {
			err = ErrHashNotFound
		}
		if err == nil {
			data = []byte(val)
		}
		return err
	})
	return
}

// putMeta associates a value with a stored hash
// N.B. this function assumes that the data associated has been properly retrieved
// and validated from the cource chain
func (dht *DHT) putMeta(key Hash, metaKey Hash, metaType string, entry Entry) (err error) {
	k := key.String()
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		_, err := tx.Get("entry:" + k)
		if err == buntdb.ErrNotFound {
			return ErrHashNotFound
		}
		mk := metaKey.String()
		var b []byte
		b, err = entry.Marshal()
		if err == nil {
			_, _, err = tx.Set("meta:"+k+":"+mk+":"+metaType, string(b), nil)
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

// getMeta retrieves values associated with hashes
func (dht *DHT) getMeta(key Hash, metaType string) (results []Entry, err error) {
	k := key.String()
	err = dht.db.View(func(tx *buntdb.Tx) error {
		_, err := tx.Get("entry:" + k)
		if err == buntdb.ErrNotFound {
			return ErrHashNotFound
		}
		results = make([]Entry, 0)
		err = tx.Ascend("meta", func(key, value string) bool {
			x := strings.Split(key, ":")
			if string(x[1]) == k && string(x[3]) == metaType {
				var entry GobEntry
				err := entry.Unmarshal([]byte(value))
				if err != nil {
					return false
				}
				results = append(results, &entry)
			}
			return true
		})
		if len(results) == 0 {
			err = fmt.Errorf("No values for %s", metaType)
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
	_, err = dht.send(n.HashAddr, PUT_REQUEST, key)
	return
}

// SendGet initiates retrieving a value from the DHT
func (dht *DHT) SendGet(key Hash) (response interface{}, err error) {
	n, err := dht.FindNodeForHash(key)
	if err != nil {
		return
	}
	response, err = dht.send(n.HashAddr, GET_REQUEST, key)
	return
}

// SendPutMeta initiates associating Meta data with particular Hash on the DHT.
// This command assumes that the data has been committed to your local chain, and the hash of that
// data is what get's sent in the MetaReq
func (dht *DHT) SendPutMeta(req MetaReq) (err error) {
	n, err := dht.FindNodeForHash(req.O)
	if err != nil {
		return
	}
	_, err = dht.send(n.HashAddr, PUTMETA_REQUEST, req)
	return
}

// SendGetMeta initiates retrieving meta data from the DHT
func (dht *DHT) SendGetMeta(query MetaQuery) (response interface{}, err error) {
	n, err := dht.FindNodeForHash(query.H)
	if err != nil {
		return
	}
	response, err = dht.send(n.HashAddr, GETMETA_REQUEST, query)
	return
}

// Send sends a message to the node
func (dht *DHT) send(to peer.ID, t MsgType, body interface{}) (response interface{}, err error) {
	return dht.h.Send(DHTProtocol, to, t, body, DHTReceiver)
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

func (dht *DHT) handlePutReqs() (err error) {
	x, err := dht.Queue.Get(10)
	if err == nil {
		for _, r := range x {
			m := r.(*Message)
			from := r.(*Message).From
			switch t := m.Body.(type) {
			case Hash:
				log.Debugf("handling put: %v", m)
				var r interface{}
				r, err = dht.h.Send(SourceProtocol, from, SRC_VALIDATE, t, SrcReceiver)
				if err != nil {
					return
				}
				// @TODO do the validation here!!!

				entry := r.(Entry)
				b, err := entry.Marshal()
				if err == nil {
					err = dht.put(t, from, b)
				}
			case MetaReq:
				log.Debugf("handling putmeta: %v", m)
				var r interface{}
				r, err = dht.h.Send(SourceProtocol, from, SRC_VALIDATE, t.M, SrcReceiver)
				if err != nil {
					return
				}
				// @TODO do the validation here!!!
				err = dht.putMeta(t.O, t.M, t.T, r.(Entry))
			default:
				err = errors.New("unexpected body type in handlePutReqs")
			}

		}
	}
	return
}

// DHTReceiver handles messages on the dht protocol
func DHTReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	switch m.Type {
	case PUT_REQUEST:
		switch m.Body.(type) {
		case Hash:
			err = h.dht.Queue.Put(m)
			if err == nil {
				response = "queued"
			}
		default:
			err = ErrDHTExpectedHashInBody
		}
		return
	case GET_REQUEST:
		switch t := m.Body.(type) {
		case Hash:
			var b []byte
			b, err = h.dht.get(t)
			if err == nil {
				var e GobEntry
				err = e.Unmarshal(b)
				if err == nil {
					response = &e
				}
			}

		default:
			err = ErrDHTExpectedHashInBody
		}
		return
	case PUTMETA_REQUEST:
		switch t := m.Body.(type) {
		case MetaReq:
			err = h.dht.exists(t.O)
			if err == nil {
				err = h.dht.Queue.Put(m)
				if err == nil {
					response = "queued"
				}
			}

		default:
			err = ErrDHTExpectedMetaInBody
		}
	case GETMETA_REQUEST:
		switch t := m.Body.(type) {
		case MetaQuery:
			response, err = h.dht.getMeta(t.H, t.T)
		default:
			err = ErrDHTExpectedMetaInBody
		}

	default:
		err = fmt.Errorf("message type %d not in holochain-dht protocol", int(m.Type))
	}
	return
}

// StartDHT initiates listening for DHT protocol messages on the node
func (dht *DHT) StartDHT() (err error) {
	err = dht.h.node.StartProtocol(dht.h, DHTProtocol, DHTReceiver)
	if err == nil {
		e := dht.h.BSget()
		if e != nil {
			log.Infof("error in BSget: %s", e.Error())
		}
	}
	return
}
