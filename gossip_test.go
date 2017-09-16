package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

/*
@TODO add setup for gossip that adds entry and meta entry so we have something
to gossip about.  Currently test is ActionReceiver test

func TestGossipReceiver(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)
	h.dht.SetupDHT()

}*/

func TestGetGossipers(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)
	dht := h.dht
	Convey("should return an empty list if none availabled", t, func() {
		glist, err := dht.getGossipers()
		So(err, ShouldBeNil)
		So(len(glist), ShouldEqual, 0)
	})

	start := 0
	testPeerCount := 20
	peers := []peer.ID{}
	peers = addTestPeers(h, peers, start, testPeerCount)

	var err error
	var glist []peer.ID
	Convey("should return all peers when neighborhood size is 0", t, func() {
		So(h.nucleus.dna.DHTConfig.NeighborhoodSize, ShouldEqual, 0)
		glist, err = dht.getGossipers()
		So(err, ShouldBeNil)
		So(len(glist), ShouldEqual, testPeerCount)
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
}

func TestGetFindGossiper(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)
	dht := h.dht
	Convey("FindGossiper should start empty", t, func() {
		_, err := dht.FindGossiper()
		So(err, ShouldEqual, ErrDHTErrNoGossipersAvailable)

	})

	fooAddr, _ := makePeer("peer_foo")

	Convey("UpdateGossiper to 0 should add the gossiper", t, func() {
		err := dht.UpdateGossiper(fooAddr, 0)
		So(err, ShouldBeNil)
	})

	Convey("FindGossiper should return the gossiper", t, func() {
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
	defer CleanupTestDir(d)
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
	dht.simHandleChangeReqs()

	someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
	e = GobEntry{C: someData}
	_, hd, _ = h.NewEntry(now, "profile", &e)
	profileHash := hd.EntryLink

	ee := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s"},{"Link":"%s"},{"Tag":"4stars"}]}`, hash.String(), profileHash.String())}
	_, le, _ := h.NewEntry(time.Now(), "rating", &ee)
	lr := LinkReq{Base: hash, Links: le.EntryLink}

	m2 := h.node.NewMessage(LINK_REQUEST, lr)
	ActionReceiver(h, m2)
	h.dht.simHandleChangeReqs()

	Convey("fingerprints for messages should exist", t, func() {
		f, _ := m1.Fingerprint()
		r, _ := dht.HaveFingerprint(f)
		So(r, ShouldBeTrue)
		f, _ = m1.Fingerprint()
		r, _ = dht.HaveFingerprint(f)
		So(r, ShouldBeTrue)
	})

	Convey("Idx should be 5 after puts", t, func() {
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
		So(puts[0].idx, ShouldEqual, 1)
		So(puts[1].idx, ShouldEqual, 2)

		puts, err = dht.GetPuts(4)
		So(err, ShouldBeNil)
		So(len(puts), ShouldEqual, 1)
		So(fmt.Sprintf("%v", puts[0].M), ShouldEqual, fmt.Sprintf("%v", *m2))
		So(puts[0].idx, ShouldEqual, 4)
	})
}

func TestGossip(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)
	dht := h.dht

	idx, _ := dht.GetIdx()
	dht.UpdateGossiper(h.node.HashAddr, idx)

	Convey("gossip should send a request", t, func() {
		var err error
		err = dht.gossip()
		So(err, ShouldBeNil)
	})
}

func TestPeerLists(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

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
