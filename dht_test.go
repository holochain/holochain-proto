package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
	"time"
)

func TestNewDHT(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
	var h Holochain
	h.rootPath = d
	os.MkdirAll(h.DBPath(), os.ModePerm)

	dht := NewDHT(&h)
	Convey("It should initialize the DHT struct", t, func() {
		So(dht.h, ShouldEqual, &h)
		So(fileExists(h.DBPath()+"/"+DHTStoreFileName), ShouldBeTrue)
	})
}

func TestSetupDHT(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	err := h.dht.SetupDHT()
	Convey("it should add the holochain ID to the DHT", t, func() {
		So(err, ShouldBeNil)
		ID := h.DNAHash()
		So(h.dht.exists(ID), ShouldBeNil)
		_, et, status, err := h.dht.get(h.dnaHash)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, LIVE)
		So(et, ShouldEqual, DNAEntryType)

	})

	Convey("it should push the agent entry to the DHT at genesis time", t, func() {
		data, et, status, err := h.dht.get(h.agentHash)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, LIVE)
		So(et, ShouldEqual, AgentEntryType)

		var e Entry
		e, _, _ = h.chain.GetEntry(h.agentHash)

		var b []byte
		b, _ = e.Marshal()

		So(string(data), ShouldEqual, string(b))
	})

	Convey("it should push the key to the DHT at genesis time", t, func() {
		keyHash, _ := NewHash(peer.IDB58Encode(h.id))
		data, et, status, err := h.dht.get(keyHash)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, LIVE)
		So(et, ShouldEqual, KeyEntryType)
		So(string(data), ShouldEqual, string([]byte(h.id)))

	})
}

func TestPutGet(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	dht := h.dht
	var id peer.ID = h.id
	Convey("It should store and retrieve", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		err := dht.put(nil, "someType", hash, id, []byte("some value"), LIVE)
		So(err, ShouldBeNil)

		data, entryType, status, err := dht.get(hash)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, LIVE)

		hash, _ = NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
		data, entryType, _, err = dht.get(hash)
		So(data, ShouldBeNil)
		So(entryType, ShouldEqual, "")
		So(err, ShouldEqual, ErrHashNotFound)
	})

}

func TestPutGetMeta(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
	var h Holochain
	h.rootPath = d
	os.MkdirAll(h.DBPath(), os.ModePerm)
	dht := NewDHT(&h)
	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	metaHash1, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
	metaHash2, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh4")
	Convey("It should fail if hash doesn't exist", t, func() {
		e1 := GobEntry{C: "some data"}
		err := dht.putMeta(nil, hash, metaHash1, "someType", &e1)
		So(err, ShouldEqual, ErrHashNotFound)

		v, err := dht.getMeta(hash, "someType")
		So(v, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	var id peer.ID
	err := dht.put(nil, "someType", hash, id, []byte("some value"), LIVE)
	if err != nil {
		panic(err)
	}

	Convey("It should store and retrieve meta values on a hash", t, func() {
		data, err := dht.getMeta(hash, "someType")
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "No values for someType")

		e1 := GobEntry{C: "value 1"}
		err = dht.putMeta(nil, hash, metaHash1, "someType", &e1)
		So(err, ShouldBeNil)

		e2 := GobEntry{C: "value 2"}
		err = dht.putMeta(nil, hash, metaHash2, "someType", &e2)
		So(err, ShouldBeNil)

		e3 := GobEntry{C: "value 3"}
		err = dht.putMeta(nil, hash, metaHash1, "otherType", &e3)
		So(err, ShouldBeNil)

		data, err = dht.getMeta(hash, "someType")
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 2)
		m := data[0]

		So(m.E.Content(), ShouldEqual, "value 1")
		m = data[1]
		So(m.E.Content(), ShouldEqual, "value 2")

		data, err = dht.getMeta(hash, "otherType")
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)
		So(data[0].E.Content(), ShouldEqual, "value 3")
		So(data[0].H, ShouldEqual, metaHash1.String())
	})
}

func TestFindNodeForHash(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("It should find a node", t, func() {

		// for now the node it finds is ourself for any hash because we haven't implemented
		// anything about neighborhoods or other nodes...
		hash, err := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		if err != nil {
			panic(err)
		}
		node, err := h.dht.FindNodeForHash(hash)
		So(err, ShouldBeNil)
		So(node.HashAddr.Pretty(), ShouldEqual, h.id.Pretty())
	})
}

func TestSend(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	node, err := NewNode("/ip4/127.0.0.1/tcp/1234", h.id, h.Agent().PrivKey())
	if err != nil {
		panic(err)
	}
	defer node.Close()

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("send GET_REQUEST message for non existent hash should get error", t, func() {
		r, err := h.dht.send(node.HashAddr, GET_REQUEST, GetReq{H: hash})
		So(r, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "4"}
	_, hd, err := h.NewEntry(now, "myData", &e)
	if err != nil {
		panic(err)
	}

	// publish the entry data to the dht
	hash = hd.EntryLink

	Convey("after a handled PUT_REQUEST data should be stored in DHT", t, func() {
		r, err := h.dht.send(node.HashAddr, PUT_REQUEST, PutReq{H: hash})
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
		h.dht.simHandlePutReqs()
		hd, _ := h.chain.GetEntryHeader(hash)
		So(hd.EntryLink.Equal(&hash), ShouldBeTrue)
	})

	Convey("send GET_REQUEST message should return content", t, func() {
		r, err := h.dht.send(node.HashAddr, GET_REQUEST, GetReq{H: hash})
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", r), ShouldEqual, fmt.Sprintf("%v", &e))
	})
}

func TestDHTReceiver(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("PUT_REQUEST should fail if body isn't a hash", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, "fish")
		_, err := DHTReceiver(h, m)
		So(err.Error(), ShouldEqual, ErrDHTExpectedPutReqInBody.Error())
	})

	Convey("PUTMETA_REQUEST should fail if body not a good put meta request", t, func() {
		m := h.node.NewMessage(PUTMETA_REQUEST, "fish")
		_, err := DHTReceiver(h, m)
		So(err.Error(), ShouldEqual, ErrDHTExpectedMetaReqInBody.Error())
	})

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("PUTMETA_REQUEST should fail if hash doesn't exist", t, func() {
		me := MetaReq{O: hash, M: hash, T: "myMetaTag"}
		m := h.node.NewMessage(PUTMETA_REQUEST, me)
		_, err := DHTReceiver(h, m)
		So(err.Error(), ShouldEqual, "hash not found")
	})

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "124"}
	_, hd, _ := h.NewEntry(now, "myData", &e)
	hash = hd.EntryLink

	Convey("PUT_REQUEST should queue a valid message", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash})
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
	})

	if err := h.dht.simHandlePutReqs(); err != nil {
		panic(err)
	}
	Convey("GET_REQUEST should return the value of the has", t, func() {
		m := h.node.NewMessage(GET_REQUEST, GetReq{H: hash})
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", r), ShouldEqual, fmt.Sprintf("%v", &e))
	})

	someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
	e = GobEntry{C: someData}
	_, hd, _ = h.NewEntry(now, "profile", &e)
	Convey("PUTMETA_REQUEST should store meta values", t, func() {
		me := MetaReq{O: hash, M: hd.EntryLink, T: "myMetaTag"}
		m := h.node.NewMessage(PUTMETA_REQUEST, me)
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")

		// fake the handleputreqs
		err = h.dht.simHandlePutReqs()
		So(err, ShouldBeNil)

		// check that it got put
		meta, err := h.dht.getMeta(hash, "myMetaTag")
		So(err, ShouldBeNil)
		So(meta[0].E.Content(), ShouldEqual, someData)
		So(meta[0].H, ShouldEqual, hd.EntryLink.String())
	})

	Convey("GETMETA_REQUEST should retrieve meta values", t, func() {
		mq := MetaQuery{H: hash, T: "myMetaTag"}
		m := h.node.NewMessage(GETMETA_REQUEST, mq)
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		results := r.(MetaQueryResp)
		So(results.Entries[0].E.Content(), ShouldEqual, someData)
	})

	Convey("GOSSIP_REQUEST should request and advertise data by idx", t, func() {
		g := GossipReq{MyIdx: 1, YourIdx: 2}
		m := h.node.NewMessage(GOSSIP_REQUEST, g)
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		gr := r.(Gossip)
		So(len(gr.Puts), ShouldEqual, 4)
	})
}

func TestGossiper(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	dht := h.dht
	Convey("FindGossiper should start empty", t, func() {
		_, err := dht.FindGossiper()
		So(err, ShouldEqual, ErrDHTErrNoGossipersAvailable)

	})

	Convey("UpdateGossiper should add a gossiper", t, func() {
		idx, _ := dht.GetIdx()
		err := dht.UpdateGossiper(h.node.HashAddr, idx)
		So(err, ShouldBeNil)
	})

	Convey("GetGossiper should return the gossiper idx", t, func() {
		idx, err := dht.GetGossiper(h.node.HashAddr)
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 3)
	})

	Convey("FindGossiper should return the gossiper", t, func() {
		g, err := dht.FindGossiper()
		So(err, ShouldBeNil)
		So(g.Idx, ShouldEqual, 3)
		So(g.Id, ShouldEqual, h.node.HashAddr)
	})

}

func TestGossipData(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	dht := h.dht
	Convey("Idx should be 3 at start (first puts are DNA, Agent & Key)", t, func() {
		var idx int
		idx, err := dht.GetIdx()
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 3)
	})

	// simulate a handled put request
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "124"}
	_, hd, _ := h.NewEntry(now, "myData", &e)
	hash := hd.EntryLink
	m1 := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash})
	DHTReceiver(h, m1)
	dht.simHandlePutReqs()

	// simulate a handled putmeta request
	me := MetaReq{O: hash, M: hd.EntryLink, T: "myMetaTag"}
	m2 := h.node.NewMessage(PUTMETA_REQUEST, me)
	DHTReceiver(h, m2)
	h.dht.simHandlePutReqs()

	Convey("Idx should be 5 after puts", t, func() {
		var idx int
		idx, err := dht.GetIdx()
		So(err, ShouldBeNil)
		So(idx, ShouldEqual, 5)
	})

	Convey("GetPuts should return a list of the puts since an index value", t, func() {
		puts, err := dht.GetPuts(0)
		So(err, ShouldBeNil)
		So(len(puts), ShouldEqual, 5)
		So(fmt.Sprintf("%v", puts[3].M), ShouldEqual, fmt.Sprintf("%v", *m1))
		So(fmt.Sprintf("%v", puts[4].M), ShouldEqual, fmt.Sprintf("%v", *m2))

		puts, err = dht.GetPuts(5)
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

func TestHandlePutReqs(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "{\"prime\":7}"}
	_, hd, err := h.NewEntry(now, "primes", &e)
	if err != nil {
		panic(err)
	}

	m := h.node.NewMessage(PUT_REQUEST, PutReq{H: hd.EntryLink})
	h.dht.puts <- m

	Convey("handle put request should pull data from source and verify it", t, func() {
		err := h.dht.simHandlePutReqs()
		So(err, ShouldBeNil)
		data, et, _, err := h.dht.get(hd.EntryLink)
		So(err, ShouldBeNil)
		So(et, ShouldEqual, "primes")
		b, _ := e.Marshal()
		So(fmt.Sprintf("%v", data), ShouldEqual, fmt.Sprintf("%v", b))
	})

}

func (dht *DHT) simHandlePutReqs() (err error) {
	m := <-dht.puts
	err = dht.handlePutReq(m)
	return
}
