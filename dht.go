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
	store map[string][]byte // the store used to persist data this node is responsible for
	h     *Holochain        // pointer to the holochain this DHT is part of
	Queue []Message
}

// Node represents a node in the network
type Node struct {
	HashAddr    Hash
	NetworkAddr string
}

// Message represents data that can be sent to node in the network
type Message struct {
	Type DHTMsgType
	Body interface{}
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
	message, err := makeMessage(PUT_REQUEST, key)
	if err != nil {
		return
	}
	err = dht.Send(n, message)
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
func (dht *DHT) Send(n *Node, msg *Message) (err error) {

	err = errors.New("not implemented")
	return
}

// FindNodeForHash gets the nearest node to the neighborhood of the hash
func (dht *DHT) FindNodeForHash(key Hash) (n *Node, err error) {

	// for now, the node it returns is self!
	var self Hash
	self, err = dht.h.TopType(KeyEntryType)
	if err != nil {
		return
	}
	var node Node
	node.HashAddr = self

	n = &node

	return
}

func makeMessage(t DHTMsgType, body interface{}) (msg *Message, err error) {
	m := Message{Type: t, Body: body}
	msg = &m
	return
}
