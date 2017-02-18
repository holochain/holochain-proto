// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// DHT implements the distributed hash table

package holochain

import (
	"errors"
)

type DHTMsgType int8

const (
	PUT_REQUEST DHTMsgType = iota
	GET_REQUEST
)

// DHT struct holds the data necessary to run the distributed hash table
type DHT struct {
	store map[Hash][]byte // the store used to persist data this node is responsible for
	h     *Holochain      // pointer to the holochain this DHT is part of
}

// Node represents a node in the network
type Node struct {
	HashAddr    Hash
	NetworkAddr string
}

func NewDHT(h *Holochain) *DHT {
	dht := DHT{store: make(map[Hash][]byte),
		h: h,
	}
	return &dht
}

func (dht *DHT) Put(key Hash) (err error) {
	n, err := FindNodeForHash(key)
	if err != nil {
		return
	}
	message, err := makeMessage(PUT_REQUEST, key)
	if err != nil {
		return
	}
	err = dht.Send(n, message)
	//	dht.store[key] = []byte("fake value")
	return
}

func (dht *DHT) Get(key Hash) (data []byte, err error) {
	data, ok := dht.store[key]
	if !ok {
		err = errors.New("No key: " + key.String())
	}
	return
}

// Send sends a message to the node
func (dht *DHT) Send(n *Node, msg string) (err error) {
	err = errors.New("not implemented")
	return
}

// FindNodeForHash gets the nearest node to the neighborhood of the hash
func FindNodeForHash(key Hash) (n *Node, err error) {
	err = errors.New("not implemented")
	return
}

func makeMessage(m DHTMsgType, body interface{}) (msg string, err error) {
	err = errors.New("not implemented")
	return
}
