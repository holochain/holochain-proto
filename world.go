// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// implements managing and storing the world model for holochain nodes

package holochain

import (
	. "github.com/holochain/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

// NodeRecord stores the necessary information about other nodes in the world model
type NodeRecord struct {
	PeerInfo pstore.PeerInfo
}

// World holds the data of a nodes' world model
type World struct {
	me          peer.ID
	nodes       map[peer.ID]*NodeRecord
	responsible map[Hash][]peer.ID
}

// NewWorld creates and empty world model
func NewWorld(me peer.ID) *World {
	world := World{me: me}
	world.nodes = make(map[peer.ID]*NodeRecord)
	world.responsible = make(map[Hash][]peer.ID)
	return &world
}

// AllNodes returns a list of all the nodes in the world model.
func (world *World) AllNodes() (nodes []peer.ID, err error) {
	nodes = make([]peer.ID, len(world.nodes))

	i := 0
	for k := range world.nodes {
		nodes[i] = k
		i++
	}
	return
}

// AddNode adds a node to the world model
func (world *World) AddNode(pi pstore.PeerInfo) (err error) {
	rec := NodeRecord{PeerInfo: pi}
	world.nodes[pi.ID] = &rec
	return
}

// NodesByHash returns a sorted list of peers, including "me" by distance from a hash
func (world *World) NodesByHash(hash Hash) (nodes []peer.ID, err error) {
	nodes, err = world.AllNodes()
	if err != nil {
		return
	}
	nodes = append(nodes, world.me)
	nodes = SortClosestPeers(nodes, hash)
	return
}

/*
func (world *World) NodeRecordsByHash(hash Hash) (records []*NodeRecord, err error) {

	records = make([]*NodeRecord, len(nodes))
	i := 0
	for _, id := range nodes {
		records[i] = world.nodes[id]
		i++
	}
	return
}*/

// UpdateResponsible calculates the list of nodes believed to be responsible for a given hash
// note that if redundancy is 0 the assumption is that all nodes are responsible
func (world *World) UpdateResponsible(hash Hash, redundancy int) (responsible bool, err error) {
	if redundancy == 0 {
		world.responsible[hash] = nil
		responsible = true
	} else if redundancy > 1 {
		var nodes []peer.ID
		nodes, err = world.NodesByHash(hash)
		if err != nil {
			return
		}
		// TODO add in resilince calculations with uptime
		i := 0
		for i = 0; i < redundancy; i++ {
			if nodes[i] == world.me {
				responsible = true
				break
			}
		}
		// if me is included in the range of nodes that are close to the has
		// add this hash (and other nodes) to the responsible map
		// otherwise delete the item from the responsible map
		if responsible {
			// remove myself from the nodes list so I can add set the
			// responsible nodes
			nodes = append(nodes[:i], nodes[i+1:redundancy]...)
			world.responsible[hash] = nodes
		} else {
			delete(world.responsible, hash)
		}
	} else {
		panic("not implemented")
	}
	return
}

// Responsible returns a list of all the entries I'm responsible for holding
func (world *World) Responsible() (entries []Hash, err error) {
	entries = make([]Hash, len(world.responsible))

	i := 0
	for k := range world.responsible {
		entries[i] = k
		i++
	}
	return
}

// Overlap holds the overlap state of a node for the overlap list
type Overlap struct {
	ID        peer.ID
	IsHolding bool
}

// Overlap returns a list of all the nodes that overlap for a given hash
func (h *Holochain) Overlap(hash Hash) (overlap []peer.ID, err error) {
	if h.nucleus.dna.DHTConfig.RedundancyFactor == 0 {
		overlap, err = h.world.AllNodes()
	} else {
		overlap = h.world.responsible[hash]
	}
	return
}
