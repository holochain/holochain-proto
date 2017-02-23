// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// DHT implements the distributed hash table

package holochain

import (
	"errors"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
)

// DHT struct holds the data necessary to run the distributed hash table
type DHT struct {
	store map[string][]byte // the store used to persist data this node is responsible for
	h     *Holochain        // pointer to the holochain this DHT is part of
	Queue []Message
}

func NewDHT(h *Holochain) *DHT {
	dht := DHT{store: make(map[string][]byte),
		h: h,
	}
	return &dht
}

func (dht *DHT) Put(key Hash) (err error) {
	n, err := dht.FindNodeForHash(key)
	if err != nil {
		return
	}
	_, err = dht.Send(n.HashAddr, PUT_REQUEST, key)
	//	dht.store[key] = []byte("fake value")
	return
}

func (dht *DHT) Get(key Hash) (data []byte, err error) {
	data, ok := dht.store[key.String()]
	if !ok {
		err = errors.New("No key: " + key.String())
	}
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
		response = "queued"
		//	h.dht.Queue
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
