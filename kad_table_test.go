// This code is adapted from the libp2p project, specifically:
// https://github.com/libp2p/go-libp2p-kbucket/table_test.go
// we don't need to unify keyspaces between random strings and peer.IDs which ipfs requires.

package holochain

import (
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	tu "github.com/libp2p/go-testutil"
	. "github.com/metacurrency/holochain/hash"
	"math/rand"
	"testing"
	"time"
)

// Test basic features of the bucket struct
func TestBucket(t *testing.T) {
	b := newBucket()

	peers := make([]peer.ID, 100)
	for i := 0; i < 100; i++ {
		peers[i] = tu.RandPeerIDFatal(t)
		b.PushFront(peers[i])
	}

	localID := tu.RandPeerIDFatal(t)

	i := rand.Intn(len(peers))
	if !b.Has(peers[i]) {
		t.Errorf("Failed to find peer: %v", peers[i])
	}

	spl := b.Split(0, localID)
	llist := b.list
	for e := llist.Front(); e != nil; e = e.Next() {
		p := e.Value.(peer.ID)
		cpl := commonPrefixLen(p, localID)
		if cpl > 0 {
			t.Fatalf("Split failed. found id with cpl > 0 in 0 bucket")
		}
	}

	rlist := spl.list
	for e := rlist.Front(); e != nil; e = e.Next() {
		p := e.Value.(peer.ID)
		cpl := commonPrefixLen(p, localID)
		if cpl == 0 {
			t.Fatalf("Split failed. found id with cpl == 0 in non 0 bucket")
		}
	}
}

func TestTableCallbacks(t *testing.T) {
	local := tu.RandPeerIDFatal(t)
	m := pstore.NewMetrics()
	rt := NewRoutingTable(10, local, time.Hour, m)

	peers := make([]peer.ID, 100)
	for i := 0; i < 100; i++ {
		peers[i] = tu.RandPeerIDFatal(t)
	}

	pset := make(map[peer.ID]struct{})
	rt.PeerAdded = func(p peer.ID) {
		pset[p] = struct{}{}
	}
	rt.PeerRemoved = func(p peer.ID) {
		delete(pset, p)
	}

	rt.Update(peers[0])
	if _, ok := pset[peers[0]]; !ok {
		t.Fatal("should have this peer")
	}

	rt.Remove(peers[0])
	if _, ok := pset[peers[0]]; ok {
		t.Fatal("should not have this peer")
	}

	for _, p := range peers {
		rt.Update(p)
	}

	out := rt.ListPeers()
	for _, outp := range out {
		if _, ok := pset[outp]; !ok {
			t.Fatal("should have peer in the peerset")
		}
		delete(pset, outp)
	}

	if len(pset) > 0 {
		t.Fatal("have peers in peerset that were not in the table", len(pset))
	}
}

// Right now, this just makes sure that it doesnt hang or crash
func TestTableUpdate(t *testing.T) {
	local := tu.RandPeerIDFatal(t)
	m := pstore.NewMetrics()
	rt := NewRoutingTable(10, local, time.Hour, m)

	peers := make([]peer.ID, 100)
	for i := 0; i < 100; i++ {
		peers[i] = tu.RandPeerIDFatal(t)
	}

	// Testing Update
	for i := 0; i < 10000; i++ {
		rt.Update(peers[rand.Intn(len(peers))])
	}

	for i := 0; i < 100; i++ {
		id := tu.RandPeerIDFatal(t)
		ret := rt.NearestPeers(HashFromPeerID(id), 5)
		if len(ret) == 0 {
			t.Fatal("Failed to find node near ID.")
		}
	}
}

func TestTableFind(t *testing.T) {
	local := tu.RandPeerIDFatal(t)
	m := pstore.NewMetrics()
	rt := NewRoutingTable(10, local, time.Hour, m)

	peers := make([]peer.ID, 100)
	for i := 0; i < 5; i++ {
		peers[i] = tu.RandPeerIDFatal(t)
		rt.Update(peers[i])
	}

	t.Logf("Searching for peer: '%s'", peers[2])
	found := rt.NearestPeer(HashFromPeerID(peers[2]))
	if !(found == peers[2]) {
		t.Fatalf("Failed to lookup known node...")
	}
}

func TestTableFindMultiple(t *testing.T) {
	local := tu.RandPeerIDFatal(t)
	m := pstore.NewMetrics()
	rt := NewRoutingTable(20, local, time.Hour, m)

	peers := make([]peer.ID, 100)
	for i := 0; i < 18; i++ {
		peers[i] = tu.RandPeerIDFatal(t)
		rt.Update(peers[i])
	}

	t.Logf("Searching for peer: '%s'", peers[2])
	found := rt.NearestPeers(HashFromPeerID(peers[2]), 15)
	if len(found) != 15 {
		t.Fatalf("Got back different number of peers than we expected.")
	}
}

// Looks for race conditions in table operations. For a more 'certain'
// test, increase the loop counter from 1000 to a much higher number
// and set GOMAXPROCS above 1
func TestTableMultithreaded(t *testing.T) {
	local, _ := peer.IDB58Decode("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	m := pstore.NewMetrics()
	tab := NewRoutingTable(20, local, time.Hour, m)
	var peers []peer.ID
	for i := 0; i < 500; i++ {
		peers = append(peers, tu.RandPeerIDFatal(t))
	}

	count := 1000
	done := make(chan struct{})
	go func() {
		for i := 0; i < count; i++ {
			n := rand.Intn(len(peers))
			tab.Update(peers[n])
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < count; i++ {
			n := rand.Intn(len(peers))
			tab.Update(peers[n])
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < count; i++ {
			n := rand.Intn(len(peers))
			tab.Find(peers[n])
		}
		done <- struct{}{}
	}()
	<-done
	<-done
	<-done
}

/*
func BenchmarkUpdates(b *testing.B) {
	b.StopTimer()
	local := ConvertKey("localKey")
	m := pstore.NewMetrics()
	tab := NewRoutingTable(20, local, time.Hour, m)

	var peers []peer.ID
	for i := 0; i < b.N; i++ {
		peers = append(peers, tu.RandPeerIDFatal(b))
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tab.Update(peers[i])
	}
}

func BenchmarkFinds(b *testing.B) {
	b.StopTimer()
	local := ConvertKey("localKey")
	m := pstore.NewMetrics()
	tab := NewRoutingTable(20, local, time.Hour, m)

	var peers []peer.ID
	for i := 0; i < b.N; i++ {
		peers = append(peers, tu.RandPeerIDFatal(b))
		tab.Update(peers[i])
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tab.Find(peers[i])
	}
}
*/
