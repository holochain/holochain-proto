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

// DHT struct holds the data necessary to run the distributed hash table
type DHT struct {
	store map[string][]byte // the store used to persist data this node is responsible for
	h     *Holochain        // pointer to the holochain this DHT is part of
	Queue q.Queue
}

// NewDHT creates a new DHT structure
func NewDHT(h *Holochain) *DHT {
	dht := DHT{store: make(map[string][]byte),
		h: h,
	}
	return &dht
}

// put stores a value to the DHT store.  N.B. This call assumes that the value has already been validated
func (dht *DHT) put(key Hash, value []byte) (err error) {
	dht.store[key.String()] = value
	return
}

// get retrieves a value from the DHT store
func (dht *DHT) get(key Hash) (data []byte, err error) {
	data, ok := dht.store[key.String()]
	if !ok {
		err = errors.New("No key: " + key.String())
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
	_, err = dht.Send(n.HashAddr, PUT_REQUEST, key)
	return
}

// Send sends a message to the node
func (dht *DHT) Send(to peer.ID, t MsgType, body interface{}) (response interface{}, err error) {
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

// DHTReceiver handles messages on the dht protocol
func DHTReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	switch m.Type {
	case PUT_REQUEST:
		err = h.dht.Queue.Put(m)
		if err == nil {
			response = "queued"
		}
		return
	case GET_REQUEST:
	default:
		err = fmt.Errorf("message type %d not in holochain-dht protocol", int(m.Type))
	}
	return
}

// StartDHT initiates listening for DHT protocol messages on the node
func (dht *DHT) StartDHT() (err error) {
	return dht.h.node.StartProtocol(dht.h, DHTProtocol, DHTReceiver)
}
