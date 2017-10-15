// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//
// This code is adapted from the libp2p project, specifically:
// https://github.com/libp2p/go-libp2p-kbucket/table.go
// we don't need to unify keyspaces between random strings and peer.IDs which ipfs requires.
//----------------------------------------------------------------------------------------

package holochain

import (
	"container/list"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	. "github.com/metacurrency/holochain/hash"
	"sort"
	"sync"
	"time"
)

// RoutingTable defines the routing table.
type RoutingTable struct {

	// ID of the local peer
	local peer.ID

	// Blanket lock, refine later for better performance
	tabLock sync.RWMutex

	// latency metrics
	metrics pstore.Metrics

	// Maximum acceptable latency for peers in this cluster
	maxLatency time.Duration

	// kBuckets define all the fingers to other nodes.
	Buckets    []*Bucket
	bucketsize int

	// notification functions
	PeerRemoved func(peer.ID)
	PeerAdded   func(peer.ID)
}

// NewRoutingTable creates a new routing table with a given bucketsize, local ID, and latency tolerance.
func NewRoutingTable(bucketsize int, localID peer.ID, latency time.Duration, m pstore.Metrics) *RoutingTable {
	rt := &RoutingTable{
		Buckets:     []*Bucket{newBucket()},
		bucketsize:  bucketsize,
		local:       localID,
		maxLatency:  latency,
		metrics:     m,
		PeerRemoved: func(peer.ID) {},
		PeerAdded:   func(peer.ID) {},
	}

	return rt
}

func commonPrefixLen(a, b peer.ID) int {
	return ZeroPrefixLen(XOR([]byte(a), []byte(b)))
}

// Update adds or moves the given peer to the front of its respective bucket
// If a peer gets removed from a bucket, it is returned
func (rt *RoutingTable) Update(p peer.ID) {
	cpl := commonPrefixLen(p, rt.local)

	rt.tabLock.Lock()
	defer rt.tabLock.Unlock()
	bucketID := cpl
	if bucketID >= len(rt.Buckets) {
		bucketID = len(rt.Buckets) - 1
	}

	bucket := rt.Buckets[bucketID]
	if bucket.Has(p) {
		// If the peer is already in the table, move it to the front.
		// This signifies that it it "more active" and the less active nodes
		// Will as a result tend towards the back of the list
		bucket.MoveToFront(p)
		return
	}

	if rt.metrics.LatencyEWMA(p) > rt.maxLatency {
		// Connection doesn't meet requirements, skip!
		return
	}

	// New peer, add to bucket
	bucket.PushFront(p)
	rt.PeerAdded(p)

	// Are we past the max bucket size?
	if bucket.Len() > rt.bucketsize {
		// If this bucket is the rightmost bucket, and its full
		// we need to split it and create a new bucket
		if bucketID == len(rt.Buckets)-1 {
			rt.PeerRemoved(rt.nextBucket())
			return
		} else {
			// If the bucket cant split kick out least active node
			rt.PeerRemoved(bucket.PopBack())
			return
		}
	}
}

// Remove deletes a peer from the routing table. This is to be used
// when we are sure a node has disconnected completely.
func (rt *RoutingTable) Remove(p peer.ID) {
	rt.tabLock.Lock()
	defer rt.tabLock.Unlock()
	cpl := commonPrefixLen(p, rt.local)

	bucketID := cpl
	if bucketID >= len(rt.Buckets) {
		bucketID = len(rt.Buckets) - 1
	}

	bucket := rt.Buckets[bucketID]
	bucket.Remove(p)
	rt.PeerRemoved(p)
}

func (rt *RoutingTable) nextBucket() peer.ID {
	bucket := rt.Buckets[len(rt.Buckets)-1]
	newBucket := bucket.Split(len(rt.Buckets)-1, rt.local)
	rt.Buckets = append(rt.Buckets, newBucket)
	if newBucket.Len() > rt.bucketsize {
		return rt.nextBucket()
	}

	// If all elements were on left side of split...
	if bucket.Len() > rt.bucketsize {
		return bucket.PopBack()
	}
	return ""
}

// Find a specific peer by ID or return nil
func (rt *RoutingTable) Find(id peer.ID) peer.ID {
	srch := rt.NearestPeers(HashFromPeerID(id), 1)
	if len(srch) == 0 || srch[0] != id {
		return ""
	}
	return srch[0]
}

// NearestPeer returns a single peer that is nearest to the given Hash
func (rt *RoutingTable) NearestPeer(hash Hash) peer.ID {
	peers := rt.NearestPeers(hash, 1)
	if len(peers) > 0 {
		return peers[0]
	}

	Debugf("NearestPeer: Returning nil, table size = %d", rt.Size())
	return ""
}

func copyPeersFromList(target peer.ID, hashArr HashSorterArr, peerList *list.List) HashSorterArr {
	center := HashFromPeerID(target)
	for e := peerList.Front(); e != nil; e = e.Next() {
		h := HashFromPeerID(e.Value.(peer.ID))
		pd := HashDistance{
			Hash:     h,
			Distance: HashXORDistance(h, center),
		}
		hashArr = append(hashArr, &pd)
	}
	return hashArr
}

func SortClosestPeers(peers []peer.ID, target Hash) []peer.ID {
	var hsarr HashSorterArr
	for _, p := range peers {
		h := HashFromPeerID(p)
		hd := &HashDistance{
			Hash:     h,
			Distance: HashXORDistance(h, target),
		}
		hsarr = append(hsarr, hd)
	}
	sort.Sort(hsarr)
	var out []peer.ID
	for _, p := range hsarr {
		out = append(out, PeerIDFromHash(p.Hash.(Hash)))
	}
	return out
}

// NearestPeers returns a list of the 'count' closest peers to the given ID
func (rt *RoutingTable) NearestPeers(hash Hash, count int) []peer.ID {
	id := PeerIDFromHash(hash)
	cpl := commonPrefixLen(id, rt.local)

	rt.tabLock.RLock()

	// Get bucket at cpl index or last bucket
	var bucket *Bucket
	if cpl >= len(rt.Buckets) {
		cpl = len(rt.Buckets) - 1
	}
	bucket = rt.Buckets[cpl]

	var hashArr HashSorterArr
	hashArr = copyPeersFromList(id, hashArr, bucket.list)
	if len(hashArr) < count {
		// In the case of an unusual split, one bucket may be short or empty.
		// if this happens, search both surrounding buckets for nearby peers
		if cpl > 0 {
			plist := rt.Buckets[cpl-1].list
			hashArr = copyPeersFromList(id, hashArr, plist)
		}

		if cpl < len(rt.Buckets)-1 {
			plist := rt.Buckets[cpl+1].list
			hashArr = copyPeersFromList(id, hashArr, plist)
		}
	}
	rt.tabLock.RUnlock()

	// Sort by distance to local peer
	sort.Sort(hashArr)

	var out []peer.ID
	for i := 0; i < count && i < hashArr.Len(); i++ {
		p := PeerIDFromHash(hashArr[i].Hash.(Hash))
		out = append(out, p)
	}

	return out
}

// Size returns the total number of peers in the routing table
func (rt *RoutingTable) Size() int {
	var tot int
	rt.tabLock.RLock()
	for _, buck := range rt.Buckets {
		tot += buck.Len()
	}
	rt.tabLock.RUnlock()
	return tot
}

// IsEmpty returns bool
func (rt *RoutingTable) IsEmpty() (empty bool) {
	rt.tabLock.RLock()
	empty = true
	for _, buck := range rt.Buckets {
		if buck.Len() > 0 {
			empty = false
			break
		}
	}
	rt.tabLock.RUnlock()
	return
}

// ListPeers takes a RoutingTable and returns a list of all peers from all buckets in the table.
// NOTE: This is potentially unsafe... use at your own risk
func (rt *RoutingTable) ListPeers() []peer.ID {
	var peers []peer.ID
	rt.tabLock.RLock()
	for _, buck := range rt.Buckets {
		peers = append(peers, buck.Peers()...)
	}
	rt.tabLock.RUnlock()
	return peers
}

// Print prints a descriptive statement about the provided RoutingTable
func (rt *RoutingTable) Print() {
	fmt.Printf("Routing Table, bs = %d, Max latency = %d\n", rt.bucketsize, rt.maxLatency)
	rt.tabLock.RLock()

	for i, b := range rt.Buckets {
		fmt.Printf("\tbucket: %d\n", i)

		b.lk.RLock()
		for e := b.list.Front(); e != nil; e = e.Next() {
			p := e.Value.(peer.ID)
			fmt.Printf("\t\t- %s %s\n", p.Pretty(), rt.metrics.LatencyEWMA(p).String())
		}
		b.lk.RUnlock()
	}
	rt.tabLock.RUnlock()
}
