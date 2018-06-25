// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// implements managing and storing the world model for holochain nodes

package holochain

import (
	"errors"
	. "github.com/HC-Interns/holochain-proto/hash"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	"sync"
)

// NodeRecord stores the necessary information about other nodes in the world model
type NodeRecord struct {
	PeerInfo  pstore.PeerInfo
	PubKey    ic.PubKey
	IsHolding map[Hash]bool
}

// World holds the data of a nodes' world model
type World struct {
	me          peer.ID
	nodes       map[peer.ID]*NodeRecord
	responsible map[Hash][]peer.ID
	ht          HashTable
	log         *Logger

	lk sync.RWMutex
}

var ErrNodeNotFound = errors.New("node not found")

// NewWorld creates and empty world model
func NewWorld(me peer.ID, ht HashTable, logger *Logger) *World {
	world := World{me: me}
	world.nodes = make(map[peer.ID]*NodeRecord)
	world.responsible = make(map[Hash][]peer.ID)
	world.ht = ht
	world.log = logger
	return &world
}

// GetNodeRecord returns the peer's node record
// NOTE: do not modify the contents of the returned record! not thread safe
func (world *World) GetNodeRecord(ID peer.ID) (record *NodeRecord) {
	world.lk.RLock()
	defer world.lk.RUnlock()
	record = world.nodes[ID]
	return
}

// SetNodeHolding marks a node as holding a particular hash
func (world *World) SetNodeHolding(ID peer.ID, hash Hash) (err error) {
	world.log.Logf("Setting Holding for %v of holding %v nodes:%v\n", ID, hash, world.nodes)
	world.lk.Lock()
	defer world.lk.Unlock()
	record := world.nodes[ID]
	if record == nil {
		err = ErrNodeNotFound
		return
	}
	record.IsHolding[hash] = true
	return
}

// IsHolding returns whether a node is holding a particular hash
func (world *World) IsHolding(ID peer.ID, hash Hash) (holding bool, err error) {
	world.lk.RLock()
	defer world.lk.RUnlock()
	world.log.Logf("Looking to see if %v is holding %v\n", ID, hash)
	world.log.Logf("NODES:%v\n", world.nodes)
	record := world.nodes[ID]
	if record == nil {
		err = ErrNodeNotFound
		return
	}
	holding = record.IsHolding[hash]
	return
}

// AllNodes returns a list of all the nodes in the world model.
func (world *World) AllNodes() (nodes []peer.ID, err error) {
	world.lk.RLock()
	defer world.lk.RUnlock()
	nodes, err = world.allNodes()
	return
}

func (world *World) allNodes() (nodes []peer.ID, err error) {
	nodes = make([]peer.ID, len(world.nodes))

	i := 0
	for k := range world.nodes {
		nodes[i] = k
		i++
	}
	return
}

// AddNode adds a node to the world model
func (world *World) AddNode(pi pstore.PeerInfo, pubKey ic.PubKey) (err error) {
	world.lk.Lock()
	defer world.lk.Unlock()
	rec := NodeRecord{PeerInfo: pi, PubKey: pubKey, IsHolding: make(map[Hash]bool)}
	world.nodes[pi.ID] = &rec
	return
}

// NodesByHash returns a sorted list of peers, including "me" by distance from a hash
func (world *World) nodesByHash(hash Hash) (nodes []peer.ID, err error) {
	nodes, err = world.allNodes()
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
	world.lk.Lock()
	defer world.lk.Unlock()
	var nodes []peer.ID
	if redundancy == 0 {
		world.responsible[hash] = nil
		responsible = true
	} else if redundancy > 1 {
		nodes, err = world.nodesByHash(hash)
		if err != nil {
			return
		}
		// TODO add in resilince calculations with uptime
		// see https://waffle.io/Holochain/holochain-proto/cards/5af33c5b8daa2d001cd1d051
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
			world.log.Logf("Number of nodes: %d, Nodes:%v\n", len(nodes), nodes)
			max := len(nodes)
			if max > redundancy {
				max = redundancy
			}
			nodes = append(nodes[:i], nodes[i+1:max]...)
			world.responsible[hash] = nodes
			world.log.Logf("Responsible for %v: %v", hash, nodes)

		} else {
			delete(world.responsible, hash)
		}
	} else {
		panic("resiliency=1 not implemented")
	}
	return
}

// Responsible returns a list of all the entries I'm responsible for holding
func (world *World) Responsible() (entries []Hash, err error) {
	world.lk.RLock()
	defer world.lk.RUnlock()
	entries = make([]Hash, len(world.responsible))

	i := 0
	for k := range world.responsible {
		entries[i] = k
		i++
	}
	return
}

// Overlap returns a list of all the nodes that overlap for a given hash
func (h *Holochain) Overlap(hash Hash) (overlap []peer.ID, err error) {
	h.world.lk.RLock()
	defer h.world.lk.RUnlock()
	if h.nucleus.dna.DHTConfig.RedundancyFactor == 0 {
		overlap, err = h.world.allNodes()
	} else {
		overlap = h.world.responsible[hash]
	}
	return
}

func myHashes(h *Holochain) (hashes []Hash) {
	h.dht.Iterate(func(hash Hash) bool {
		hashes = append(hashes, hash)
		return true
	})
	return
}

func HoldingTask(h *Holochain) {
	//	coholders := make(map[*NodeRecord][]Hash)

	// to protect against crashes from background routines after close
	if h.dht == nil {
		return
	}
	hashes := myHashes(h)
	for _, hash := range hashes {
		if hash.String() == h.dnaHash.String() {
			continue
		}

		// TODO forget the hashes we are no longer responsible for
		// https://waffle.io/Holochain/holochain-proto/cards/5af33e3b361c27001d5348c6
		// TODO this really shouldn't be called in the holding task
		//     but instead should be called with the Node list or hash list changes.
		h.world.UpdateResponsible(hash, h.RedundancyFactor())
		h.world.log.Logf("HoldingTask: updated %v\n", hash)
		overlap, err := h.Overlap(hash)
		if err == nil {
			h.world.log.Logf("HoldingTask: sending put requests to %d nodes\n", len(overlap))

			for _, node := range overlap {
				// to protect against crashes from background routines after close
				if h.node == nil {
					return
				}
				/*rec := h.world.GetNodeRecord(node)
				/*				hashes := coholders[rec]
								coholders[rec] = append(hashes, hash)
				*/
				h.world.log.Logf("HoldingTask: PUT_REQUEST sent to %v\n", node)
				msg := h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: hash})
				h.dht.sendChange(node, msg)
			}
		}
	}

	/*	for rec, hashes := range coholders {

		}
	*/
}
