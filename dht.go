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
)

var ErrDHTExpectedHashInBody error = errors.New("expected hash")
var ErrDHTExpectedMetaInBody error = errors.New("expected meta struct")

// DHT struct holds the data necessary to run the distributed hash table
type DHT struct {
	store     map[string][]byte  // the store used to persist data this node is responsible for
	metastore map[string][]Meta  // the store used to persist putMetas
	sources   map[string]peer.ID // the store used to persist source data
	h         *Holochain         // pointer to the holochain this DHT is part of
	Queue     q.Queue            // a queue for incoming puts
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
		store:     make(map[string][]byte),
		metastore: make(map[string][]Meta),
		sources:   make(map[string]peer.ID),
		h:         h,
	}
	return &dht
}

// put stores a value to the DHT store
// N.B. This call assumes that the value has already been validated
func (dht *DHT) put(key Hash, src peer.ID, value []byte) (err error) {
	k := key.String()
	dht.store[k] = value
	dht.sources[k] = src
	return
}

// exists checks for the existence of the hash in the store
func (dht *DHT) exists(key Hash) (err error) {
	_, ok := dht.store[key.String()]
	if !ok {
		err = ErrHashNotFound
	}
	return
}

// returns the source of a given hash
func (dht *DHT) source(hash Hash) (id peer.ID, err error) {
	id, ok := dht.sources[hash.String()]
	if !ok {
		err = ErrHashNotFound
	}
	return
}

// get retrieves a value from the DHT store
func (dht *DHT) get(key Hash) (data []byte, err error) {
	err = dht.exists(key)
	if err == nil {
		data, _ = dht.store[key.String()]
	}
	return
}

// putMeta associates a value with a stored hash
// N.B. this function assumes that the data associated has been properly retrieved
// and validated from the cource chain
func (dht *DHT) putMeta(key Hash, metaKey Hash, metaType string, value []byte) (err error) {
	err = dht.exists(key)
	if err == nil {
		m := Meta{H: metaKey, T: metaType, V: value}
		ks := key.String()
		v, ok := dht.metastore[ks]
		if !ok {
			dht.metastore[ks] = []Meta{m}
		} else {
			dht.metastore[ks] = append(v, m)
		}
	}
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
func (dht *DHT) getMeta(key Hash, metaType string) (results []Meta, err error) {
	err = dht.exists(key)
	if err == nil {
		ks := key.String()
		v, ok := dht.metastore[ks]
		if ok {
			results = filter(v, func(m *Meta) bool { return m.T == metaType })
		}
		if !ok || len(results) == 0 {
			err = fmt.Errorf("No values for %s", metaType)
		}
	}
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
				entry := r.(Entry)
				b, err := entry.Marshal()
				if err == nil {
					err = dht.putMeta(t.O, t.M, t.T, b)
				}
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
	return dht.h.node.StartProtocol(dht.h, DHTProtocol, DHTReceiver)
}
