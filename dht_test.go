package holochain

import (
	"bytes"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestPutGet(t *testing.T) {
	var h Holochain
	dht := NewDHT(&h)
	var id peer.ID
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
	var h Holochain
	dht := NewDHT(&h)
	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	metaHash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")

	Convey("It should fail if hash doesn't exist", t, func() {
		err := dht.putMeta(hash, metaHash, "someType", []byte("some data"))
		So(err, ShouldEqual, ErrHashNotFound)

		_, err = dht.getMeta(hash, "someType")
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

		err = dht.putMeta(hash, metaHash, "someType", []byte("value 1"))
		So(err, ShouldBeNil)

		err = dht.putMeta(hash, metaHash, "someType", []byte("value 2"))
		So(err, ShouldBeNil)

		err = dht.putMeta(hash, metaHash, "otherType", []byte("value 3"))
		So(err, ShouldBeNil)

		data, err = dht.getMeta(hash, "someType")
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 2)
		m := data[0]
		So(m.H.String(), ShouldEqual, "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
		So(string(m.V), ShouldEqual, "value 1")
		m = data[1]
		So(m.H.String(), ShouldEqual, "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
		So(string(m.V), ShouldEqual, "value 2")

		data, err = dht.getMeta(hash, "otherType")
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)
		So(string(data[0].V), ShouldEqual, "value 3")

	})
}

func TestNewDHT(t *testing.T) {
	var h Holochain

	dht := NewDHT(&h)
	Convey("It should initialize the DHT struct", t, func() {
		So(dht.h, ShouldEqual, &h)
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
		r, err := h.dht.send(node.HashAddr, GET_REQUEST, hash)
		So(r, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "some data"}
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
		r, err := h.dht.send(node.HashAddr, PUT_REQUEST, hash)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
		So(h.dht.Queue.Len(), ShouldEqual, 1)
		messages, err := h.dht.Queue.Get(1)
		So(err, ShouldBeNil)
		m := messages[0].(*Message)
		So(m.Type, ShouldEqual, PUT_REQUEST)
		hs := m.Body.(Hash)
		So(hs.String(), ShouldEqual, hash.String())
	})

	Convey("after a handled PUT_REQUEST data should be stored in DHT", t, func() {
		r, err := h.dht.send(node.HashAddr, PUT_REQUEST, hash)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
		h.dht.handlePutReqs()
		hd, _ := h.chain.GetEntryHeader(hash)
		So(hd.EntryLink.String(), ShouldEqual, hash.String())
	})

	Convey("send GET_REQUEST message should return content", t, func() {
		r, err := h.dht.send(node.HashAddr, GET_REQUEST, hash)
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
		So(err.Error(), ShouldEqual, "expected hash")
	})

	Convey("PUTMETA_REQUEST should fail if body not a good put meta request", t, func() {
		m := h.node.NewMessage(PUTMETA_REQUEST, "fish")
		_, err := DHTReceiver(h, m)
		So(err.Error(), ShouldEqual, "expected meta struct")
	})

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("PUTMETA_REQUEST should fail if hash doesn't exist", t, func() {
		me := MetaReq{O: hash, M: hash, T: "myMetaType"}
		m := h.node.NewMessage(PUTMETA_REQUEST, me)
		_, err := DHTReceiver(h, m)
		So(err.Error(), ShouldEqual, "hash not found")
	})

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "some data"}
	_, hd, _ := h.NewEntry(now, "myData", &e)
	hash = hd.EntryLink

	Convey("PUT_REQUEST should queue a valid message", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, hash)
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
	})

	if err := h.dht.handlePutReqs(); err != nil {
		panic(err)
	}
	Convey("GET_REQUEST should return the value of the has", t, func() {
		m := h.node.NewMessage(GET_REQUEST, hash)
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", r), ShouldEqual, fmt.Sprintf("%v", &e))
	})

	Convey("PUTMETA_REQUEST should store meta values", t, func() {
		e := GobEntry{C: "some meta data"}
		_, hd, _ := h.NewEntry(now, "myMetaData", &e)
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
		So(meta[0].H.Equal(&me.M), ShouldBeTrue)
		So(meta[0].T, ShouldEqual, me.T)
		b, _ := e.Marshal()
		So(bytes.Equal(meta[0].V, b), ShouldBeTrue)
	})
}

func TestHandlePutReqs(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "some data"}
	_, hd, err := h.NewEntry(now, "myData", &e)
	if err != nil {
		panic(err)
	}

	m := h.node.NewMessage(PUT_REQUEST, hd.EntryLink)
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
