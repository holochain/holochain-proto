package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

/*
@TODO add setup for gossip that adds entry and meta entry so we have something
to gossip about.  Currently test is DHTReceiver test

func TestGossipReceiver(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	h.dht.SetupDHT()

}*/

func TestGetFindGossiper(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	dht := h.dht
	Convey("FindGossiper should start empty", t, func() {
		_, err := dht.FindGossiper()
		So(err, ShouldEqual, ErrDHTErrNoGossipersAvailable)

	})

	fooAddr, _ := makePeer("peer_foo")

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
		So(g.Idx, ShouldEqual, 92)
		So(g.Id, ShouldEqual, fooAddr)
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
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
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
		So(msg1.Body.(PutReq).H.String(), ShouldEqual, peer.IDB58Encode(h.id))
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
	DHTReceiver(h, m1)
	dht.simHandleChangeReqs()

	someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
	e = GobEntry{C: someData}
	_, hd, _ = h.NewEntry(now, "profile", &e)
	profileHash := hd.EntryLink

	ee := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s"},{"Link":"%s"},{"Tag":"4stars"}]}`, hash.String(), profileHash.String())}
	_, le, _ := h.NewEntry(time.Now(), "rating", &ee)
	lr := LinkReq{Base: hash, Links: le.EntryLink}

	m2 := h.node.NewMessage(LINK_REQUEST, lr)
	DHTReceiver(h, m2)
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

		puts, err = dht.GetPuts(4)
		So(err, ShouldBeNil)
		So(len(puts), ShouldEqual, 1)
		So(fmt.Sprintf("%v", puts[0].M), ShouldEqual, fmt.Sprintf("%v", *m2))
	})
}

func TestGossip(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	dht := h.dht

	idx, _ := dht.GetIdx()
	dht.UpdateGossiper(h.node.HashAddr, idx)

	Convey("gossip should send a request", t, func() {
		var err error
		err = dht.gossip()
		So(err, ShouldBeNil)
	})
}
