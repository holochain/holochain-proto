// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// DHT implements the distributed hash table

package holochain

import (
	"errors"
)

type DHT struct {
	store map[Hash][]byte
}

func NewDHT() *DHT {
	dht := DHT{store: make(map[Hash][]byte)}
	return &dht
}

func (dht *DHT) Put(key Hash) (err error) {
	dht.store[key] = []byte("fake value")
	return
}

func (dht *DHT) Get(key Hash) (data []byte, err error) {
	data, ok := dht.store[key]
	if !ok {
		err = errors.New("No key: " + key.String())
	}
	return
}
