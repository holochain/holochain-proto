package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestNewDHT(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
	var h Holochain
	h.path = d
	dht := NewDHT(&h)
	Convey("It should initialize the DHT struct", t, func() {
		So(dht.h, ShouldEqual, &h)
		So(fileExists(h.path+"/dht.db"), ShouldBeTrue)
	})
}

func TestPutGet(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	dht := h.dht
	var id peer.ID = h.node.HashAddr
	Convey("It should store and retrieve", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		err := dht.put(hash, id, []byte("some value"))
		So(err, ShouldBeNil)
		data, err := dht.get(hash)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		hash, _ = NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		data, err = dht.get(hash)
		So(data, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})
}

func TestPutGetMeta(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
	var h Holochain
	h.path = d
	dht := NewDHT(&h)
	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	metaHash1, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
	metaHash2, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh4")
	Convey("It should fail if hash doesn't exist", t, func() {
		e1 := GobEntry{C: "some data"}
		err := dht.putMeta(hash, metaHash1, "someType", &e1)
		So(err, ShouldEqual, ErrHashNotFound)

		v, err := dht.getMeta(hash, "someType")
		So(v, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	var id peer.ID
	err := dht.put(hash, id, []byte("some value"))
	if err != nil {
		panic(err)
	}

	Convey("It should store and retrieve meta values on a hash", t, func() {
		data, err := dht.getMeta(hash, "someType")
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "No values for someType")

		e1 := GobEntry{C: "value 1"}
		err = dht.putMeta(hash, metaHash1, "someType", &e1)
		So(err, ShouldBeNil)

		e2 := GobEntry{C: "value 2"}
		err = dht.putMeta(hash, metaHash2, "someType", &e2)
		So(err, ShouldBeNil)

		e3 := GobEntry{C: "value 3"}
		err = dht.putMeta(hash, metaHash1, "otherType", &e3)
		So(err, ShouldBeNil)

		data, err = dht.getMeta(hash, "someType")
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 2)
		m := data[0]

		So(m.Content(), ShouldEqual, "value 1")
		m = data[1]
		So(m.Content(), ShouldEqual, "value 2")

		data, err = dht.getMeta(hash, "otherType")
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)
		So(data[0].Content(), ShouldEqual, "value 3")
	})
}

func TestFindNodeForHash(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("It should find a node", t, func() {

		// for now the node it finds is ourself for any hash because we haven't implemented
		// anything about neighborhoods or other nodes...
		self := h.node.HashAddr
		hash, err := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		if err != nil {
			panic(err)
		}
		node, err := h.dht.FindNodeForHash(hash)
		So(err, ShouldBeNil)
		So(node.HashAddr.Pretty(), ShouldEqual, self.Pretty())
	})
}

func TestSend(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	node, err := NewNode("/ip4/127.0.0.1/tcp/1234", h.Agent().PrivKey())
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

	Convey("before send PUT_REQUEST message queue should be empty", t, func() {
		So(h.dht.Queue.Len(), ShouldEqual, 0)
	})

	Convey("after send PUT_REQUEST message queue should have the message in it", t, func() {
		r, err := h.dht.send(node.HashAddr, PUT_REQUEST, PutReq{H: hash})
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
		So(h.dht.Queue.Len(), ShouldEqual, 1)
		messages, err := h.dht.Queue.Get(1)
		So(err, ShouldBeNil)
		m := messages[0].(*Message)
		So(m.Type, ShouldEqual, PUT_REQUEST)
		pr := m.Body.(PutReq)
		So(pr.H.Equal(&hash), ShouldBeTrue)
	})

	Convey("after a handled PUT_REQUEST data should be stored in DHT", t, func() {
		r, err := h.dht.send(node.HashAddr, PUT_REQUEST, PutReq{H: hash})
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
		h.dht.handlePutReqs()
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
		me := MetaReq{O: hash, M: hash, T: "myMetaType"}
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

	if err := h.dht.handlePutReqs(); err != nil {
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
		me := MetaReq{O: hash, M: hd.EntryLink, T: "myMetaType"}
		m := h.node.NewMessage(PUTMETA_REQUEST, me)
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")

		// fake the handleputreqs
		err = h.dht.handlePutReqs()
		So(err, ShouldBeNil)

		// check that it got put
		meta, err := h.dht.getMeta(hash, "myMetaType")
		So(err, ShouldBeNil)
		So(meta[0].Content(), ShouldEqual, someData)
	})

	Convey("GETMETA_REQUEST should retrieve meta values", t, func() {
		mq := MetaQuery{H: hash, T: "myMetaType"}
		m := h.node.NewMessage(GETMETA_REQUEST, mq)
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		results := r.([]Entry)
		So(results[0].Content(), ShouldEqual, someData)
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
	err = h.dht.Queue.Put(m)

	Convey("handle put request should pull data from source and verify it", t, func() {
		err := h.dht.handlePutReqs()
		So(err, ShouldBeNil)
		data, err := h.dht.get(hd.EntryLink)
		So(err, ShouldBeNil)
		b, _ := e.Marshal()
		So(fmt.Sprintf("%v", data), ShouldEqual, fmt.Sprintf("%v", b))
	})

}
