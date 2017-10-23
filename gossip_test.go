package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	ma "github.com/multiformats/go-multiaddr"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

/*
@TODO add setup for gossip that adds entry and meta entry so we have something
to gossip about.  Currently test is ActionReceiver test

func TestGossipReceiver(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h,d)
	h.dht.SetupDHT()

}*/

func TestGetGossipers(t *testing.T) {
	nodesCount := 20
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes
	h := nodes[0]
	dht := h.dht
	Convey("should return an empty list if none availabled", t, func() {
		glist, err := dht.getGossipers()
		So(err, ShouldBeNil)
		So(len(glist), ShouldEqual, 0)
	})

	starConnect(t, mt.ctx, nodes, nodesCount)

	var err error
	var glist []peer.ID
	Convey("should return all peers when neighborhood size is 0", t, func() {
		So(h.nucleus.dna.DHTConfig.NeighborhoodSize, ShouldEqual, 0)
		glist, err = dht.getGossipers()
		So(err, ShouldBeNil)
		So(len(glist), ShouldEqual, nodesCount-1)
	})

	Convey("should return neighborhood size peers when neighborhood size is not 0", t, func() {
		h.nucleus.dna.DHTConfig.NeighborhoodSize = 5
		glist, err = dht.getGossipers()
		So(err, ShouldBeNil)
		So(len(glist), ShouldEqual, 5)
	})

	Convey("should return list sorted by closeness to me", t, func() {
		So(h.node.Distance(glist[0]).Cmp(h.node.Distance(glist[1])), ShouldBeLessThanOrEqualTo, 0)
		So(h.node.Distance(glist[1]).Cmp(h.node.Distance(glist[2])), ShouldBeLessThanOrEqualTo, 0)
		So(h.node.Distance(glist[2]).Cmp(h.node.Distance(glist[3])), ShouldBeLessThanOrEqualTo, 0)
		So(h.node.Distance(glist[3]).Cmp(h.node.Distance(glist[4])), ShouldBeLessThanOrEqualTo, 0)
		So(h.node.Distance(glist[0]), ShouldNotEqual, h.node.Distance(glist[4]))
	})

	Convey("it should only return active gossipers.", t, func() {

		// mark one of nodes previously found as closed
		id := glist[0]
		h.node.peerstore.ClearAddrs(id)
		glist, err = dht.getGossipers()
		So(err, ShouldBeNil)
		So(len(glist), ShouldEqual, 5)

		So(glist[0].Pretty(), ShouldNotEqual, id.Pretty())
	})
}

func TestGetFindGossiper(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	dht := h.dht
	Convey("FindGossiper should start empty", t, func() {
		_, err := dht.FindGossiper()
		So(err, ShouldEqual, ErrDHTErrNoGossipersAvailable)

	})

	Convey("AddGossiper of ourselves should not add the gossiper", t, func() {
		err := dht.AddGossiper(h.node.HashAddr)
		So(err, ShouldBeNil)
		_, err = dht.FindGossiper()
		So(err, ShouldEqual, ErrDHTErrNoGossipersAvailable)
	})

	fooAddr, _ := makePeer("peer_foo")
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/1234")
	if err != nil {
		panic(err)
	}
	h.node.peerstore.AddAddrs(fooAddr, []ma.Multiaddr{addr}, PeerTTL)

	Convey("AddGossiper add the gossiper", t, func() {
		err := dht.AddGossiper(fooAddr)
		So(err, ShouldBeNil)
		g, err := dht.FindGossiper()
		So(err, ShouldBeNil)
		So(g, ShouldEqual, fooAddr)
	})

	Convey("DeleteGossiper should remove a gossiper from the database", t, func() {
		err := dht.DeleteGossiper(fooAddr)
		So(err, ShouldBeNil)
		_, err = dht.FindGossiper()
		So(err, ShouldEqual, ErrDHTErrNoGossipersAvailable)
		err = dht.DeleteGossiper(fooAddr)
		So(err.Error(), ShouldEqual, "not found")
	})

	Convey("GetGossiper should return the gossiper idx", t, func() {
		idx, err := dht.GetGossiper(fooAddr)
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 0)
	})

	Convey("UpdateGossiper should add a gossiper", t, func() {
		err := dht.UpdateGossiper(fooAddr, 92)
		So(err, ShouldBeNil)
	})

	Convey("GetGossiper should return the gossiper idx", t, func() {
		idx, err := dht.GetGossiper(fooAddr)
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 92)
	})

	Convey("UpdateGossiper should ignore values less than previously stored", t, func() {
		err := dht.UpdateGossiper(fooAddr, 32)
		So(err, ShouldBeNil)
		idx, err := dht.GetGossiper(fooAddr)
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 92)
	})

	Convey("FindGossiper should return the gossiper", t, func() {
		g, err := dht.FindGossiper()
		So(err, ShouldBeNil)
		So(g, ShouldEqual, fooAddr)
	})

	Convey("UpdateGossiper should update when value greater than previously stored", t, func() {
		err := dht.UpdateGossiper(fooAddr, 132)
		So(err, ShouldBeNil)
		idx, err := dht.GetGossiper(fooAddr)
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 132)
	})

	Convey("GetIdx for self should be 2 to start with (DNA not stored)", t, func() {
		idx, err := dht.GetIdx()
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 2)
	})

	barAddr, _ := makePeer("peer_bar")

	Convey("GetGossiper should return 0 for unknown gossiper", t, func() {
		idx, err := dht.GetGossiper(barAddr)
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 0)
	})
}

func TestGossipData(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	dht := h.dht
	Convey("Idx should be 2 at start (first puts are DNA, Agent & Key but DNA put not stored)", t, func() {
		var idx int
		idx, err := dht.GetIdx()
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 2)
	})

	var msg1 Message
	var err error
	Convey("GetIdxMessage should return the message that made the change", t, func() {
		msg1, err = dht.GetIdxMessage(1)
		So(err, ShouldBeNil)
		So(msg1.Type, ShouldEqual, PUT_REQUEST)
		So(msg1.Body.(PutReq).H.String(), ShouldEqual, h.nodeIDStr)
	})

	Convey("GetFingerprint should return the index of the message that made the change", t, func() {
		f, _ := msg1.Fingerprint()
		idx, err := dht.GetFingerprint(f)
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 1)

		idx, err = dht.GetFingerprint(NullHash())
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, -1)
	})

	// simulate a handled put request
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "124"}
	_, hd, _ := h.NewEntry(now, "evenNumbers", &e)
	hash := hd.EntryLink
	m1 := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash})

	Convey("fingerprints for messages should not exist", t, func() {
		f, _ := m1.Fingerprint()
		r, _ := dht.HaveFingerprint(f)
		So(r, ShouldBeFalse)
	})
	ActionReceiver(h, m1)

	someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
	e = GobEntry{C: someData}
	_, hd, _ = h.NewEntry(now, "profile", &e)
	profileHash := hd.EntryLink

	ee := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s"},{"Link":"%s"},{"Tag":"4stars"}]}`, hash.String(), profileHash.String())}
	_, le, _ := h.NewEntry(time.Now(), "rating", &ee)
	lr := LinkReq{Base: hash, Links: le.EntryLink}

	m2 := h.node.NewMessage(LINK_REQUEST, lr)
	ActionReceiver(h, m2)

	Convey("fingerprints for messages should exist", t, func() {
		f, _ := m1.Fingerprint()
		r, _ := dht.HaveFingerprint(f)
		So(r, ShouldBeTrue)
		f, _ = m1.Fingerprint()
		r, _ = dht.HaveFingerprint(f)
		So(r, ShouldBeTrue)
	})

	Convey("Idx should be 4 after puts", t, func() {
		var idx int
		idx, err := dht.GetIdx()
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 4)
	})

	Convey("GetPuts should return a list of the puts since an index value", t, func() {
		puts, err := dht.GetPuts(0)
		So(err, ShouldBeNil)
		So(len(puts), ShouldEqual, 4)
		So(fmt.Sprintf("%v", puts[2].M), ShouldEqual, fmt.Sprintf("%v", *m1))
		So(fmt.Sprintf("%v", puts[3].M), ShouldEqual, fmt.Sprintf("%v", *m2))
		So(puts[0].Idx, ShouldEqual, 1)
		So(puts[1].Idx, ShouldEqual, 2)

		puts, err = dht.GetPuts(4)
		So(err, ShouldBeNil)
		So(len(puts), ShouldEqual, 1)
		So(fmt.Sprintf("%v", puts[0].M), ShouldEqual, fmt.Sprintf("%v", *m2))
		So(puts[0].Idx, ShouldEqual, 4)
	})
}

func TestGossip(t *testing.T) {
	nodesCount := 2
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes

	h1 := nodes[0]
	h2 := nodes[1]

	commit(h1, "oddNumbers", "3")
	commit(h1, "oddNumbers", "5")
	commit(h1, "oddNumbers", "7")

	puts1, _ := h1.dht.GetPuts(0)
	puts2, _ := h2.dht.GetPuts(0)

	Convey("Idx after puts", t, func() {
		So(len(puts1), ShouldEqual, 5)
		So(len(puts2), ShouldEqual, 2)
	})
	ringConnect(t, mt.ctx, mt.nodes, nodesCount)
	Convey("gossipWith should add the puts", t, func() {
		err := h2.dht.gossipWith(h1.nodeID)
		So(err, ShouldBeNil)
		go h2.dht.HandleGossipPuts()
		time.Sleep(time.Millisecond * 100)
		puts2, _ = h2.dht.GetPuts(0)
		So(len(puts2), ShouldEqual, 7)
	})
	commit(h1, "evenNumbers", "2")
	commit(h1, "evenNumbers", "4")

	Convey("gossipWith should add the puts", t, func() {
		err := h2.dht.gossipWith(h1.nodeID)
		So(err, ShouldBeNil)
		go h2.dht.HandleGossipPuts()
		time.Sleep(time.Millisecond * 100)
		puts2, _ = h2.dht.GetPuts(0)
		So(len(puts2), ShouldEqual, 9)
	})
}

func TestPeerLists(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("it should start with an empty blockedlist", t, func() {
		peerList, err := h.dht.getList(BlockedList)
		So(err, ShouldBeNil)
		So(len(peerList.Records), ShouldEqual, 0)
	})

	Convey("it should have peers after they're added", t, func() {
		pid1, _ := makePeer("testPeer1")
		pid2, _ := makePeer("testPeer2")
		pids := []PeerRecord{PeerRecord{ID: pid1}, PeerRecord{ID: pid2}}

		idx, _ := h.dht.GetIdx()
		err := h.dht.addToList(h.node.NewMessage(LISTADD_REQUEST, ListAddReq{ListType: BlockedList, Peers: []string{peer.IDB58Encode(pid1), peer.IDB58Encode(pid2)}}), PeerList{BlockedList, pids})
		So(err, ShouldBeNil)

		afterIdx, _ := h.dht.GetIdx()
		So(afterIdx-idx, ShouldEqual, 1)

		peerList, err := h.dht.getList(BlockedList)
		So(err, ShouldBeNil)
		So(peerList.Type, ShouldEqual, BlockedList)
		So(len(peerList.Records), ShouldEqual, 2)
		So(peerList.Records[0].ID, ShouldEqual, pid1)
		So(peerList.Records[1].ID, ShouldEqual, pid2)
	})
}

func TestGossipCycle(t *testing.T) {
	nodesCount := 2
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes
	h0 := nodes[0]
	h1 := nodes[1]
	ringConnect(t, mt.ctx, nodes, nodesCount)

	Convey("the gossip task should schedule a gossipWithRequest", t, func() {
		So(len(h0.dht.gchan), ShouldEqual, 0)
		GossipTask(h0)
		So(len(h0.dht.gchan), ShouldEqual, 1)
	})

	Convey("handling the gossipWith should result in getting puts, and a gossip back scheduled on receiving node after a delay", t, func() {

		So(len(h1.dht.gchan), ShouldEqual, 0)
		So(len(h0.dht.gossipPuts), ShouldEqual, 0)

		stop, err := handleGossipWith(h0.dht)
		So(stop, ShouldBeFalse)
		So(err, ShouldBeNil)
		// we got receivers puts back and scheduled
		So(len(h0.dht.gossipPuts), ShouldEqual, 2)

		So(len(h1.dht.gchan), ShouldEqual, 0)
		time.Sleep(GossipBackPutDelay * 3)
		// gossip back scheduled on receiver after delay
		So(len(h1.dht.gchan), ShouldEqual, 1)
	})

	Convey("gossipWith shouldn't be rentrant with respect to the same gossiper", t, func() {
		log := &h0.Config.Loggers.Gossip
		log.color, log.f = log.setupColor("%{message}")

		// if the code were rentrant the log would should the events in a different order
		ShouldLog(log, "node0_starting gossipWith <peer.ID UfY4We>\nnode0_no new puts received\nnode0_finish gossipWith <peer.ID UfY4We>, err=<nil>\nnode0_starting gossipWith <peer.ID UfY4We>\nnode0_no new puts received\nnode0_finish gossipWith <peer.ID UfY4We>, err=<nil>\n", func() {
			go h0.dht.gossipWith(h1.nodeID)
			h0.dht.gossipWith(h1.nodeID)
			time.Sleep(time.Millisecond * 100)
		})
		log.color, log.f = log.setupColor(log.Format)
	})
}

func TestGossipErrorCases(t *testing.T) {
	nodesCount := 2
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes
	h0 := nodes[0]
	h1 := nodes[1]
	ringConnect(t, mt.ctx, nodes, nodesCount)
	Convey("a rejected put should not break gossiping", t, func() {
		// inject a bad put
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqz2")
		h1.dht.put(h1.node.NewMessage(PUT_REQUEST, PutReq{H: hash}), "evenNumbers", hash, h0.nodeID, []byte("bad data"), StatusLive)
		err := h0.dht.gossipWith(h1.nodeID)
		So(err, ShouldBeNil)
		So(len(h0.dht.gossipPuts), ShouldEqual, 3)
		for i := 0; i < 3; i++ {
			stop, err := handleGossipPut(h0.dht)
			So(stop, ShouldBeFalse)
			So(err, ShouldBeNil)
		}
		err = h0.dht.gossipWith(h1.nodeID)
		So(err, ShouldBeNil)
		So(len(h0.dht.gossipPuts), ShouldEqual, 0)
	})
}

func TestGossipPropigation(t *testing.T) {
	nodesCount := 10
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes
	ringConnect(t, mt.ctx, nodes, nodesCount)
	//randConnect(t, mt.ctx, nodes, nodesCount, 7, 4)
	//starConnect(t, mt.ctx, nodes, nodesCount)
	Convey("each node should have one gossiper from the ring connect", t, func() {
		for i := 0; i < nodesCount; i++ {
			glist, err := nodes[i].dht.getGossipers()
			So(err, ShouldBeNil)
			So(len(glist), ShouldEqual, 1)
		}
	})

	Convey("each node should only have it's own puts", t, func() {
		for i := 0; i < nodesCount; i++ {
			puts, err := nodes[i].dht.GetPuts(0)
			So(err, ShouldBeNil)
			So(len(puts), ShouldEqual, 2)
		}
	})

	Convey("each node should only have everybody's puts after enough propigation time", t, func() {

		for i := 0; i < nodesCount; i++ {
			nodes[i].Config.gossipInterval = 200 * time.Millisecond
			nodes[i].StartBackgroundTasks()
		}

		start := time.Now()
		propigated := false
		ticker := time.NewTicker(210 * time.Millisecond)
		stop := make(chan bool, 1)

		go func() {
			for tick := range ticker.C {
				// abort just in case in 4 seconds (only if propgation fails)
				if tick.Sub(start) > (10 * time.Second) {
					//fmt.Printf("Aborting!")
					stop <- true
					return
				}

				propigated = true
				// check to see if the nodes have all gotten the puts yet.
				for i := 0; i < nodesCount; i++ {
					puts, _ := nodes[i].dht.GetPuts(0)
					if len(puts) < nodesCount*2 {
						propigated = false
					}
					/*					fmt.Printf("NODE%d(%s): %d:", i, nodes[i].nodeID.Pretty()[2:4], len(puts))
										for j := 0; j < len(puts); j++ {
											f, _ := puts[j].M.Fingerprint()
											fmt.Printf("%s,", f.String()[2:4])
										}
										fmt.Printf("\n              ")
										nodes[i].dht.glk.RLock()
										for k, _ := range nodes[i].dht.fingerprints {
											fmt.Printf("%s,", k)
										}
										nodes[i].dht.glk.RUnlock()
										fmt.Printf("\n    ")
										for k, _ := range nodes[i].dht.sources {
											fmt.Printf("%d,", findNodeIdx(nodes, k))
										}
										fmt.Printf("\n")
					*/
				}
				if propigated {
					stop <- true
					return
				}
				//				fmt.Printf("\n")
			}
		}()
		<-stop
		ticker.Stop()
		So(propigated, ShouldBeTrue)
	})
}

func findNodeIdx(nodes []*Holochain, id peer.ID) int {
	for i, n := range nodes {
		if id == n.nodeID {
			return i
		}
	}
	panic("bork!")
}
