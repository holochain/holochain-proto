package holochain

import (
	_ "fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestPutGet(t *testing.T) {
	var h Holochain
	dht := NewDHT(&h)
	Convey("It should store and retrieve", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		err := dht.put(hash, []byte("some value"))
		So(err, ShouldBeNil)
		data, err := dht.get(hash)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		hash, _ = NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		data, err = dht.get(hash)
		So(err.Error(), ShouldEqual, "No key: QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
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
		self, err := peer.IDFromPrivateKey(h.Agent().PrivKey())
		if err != nil {
			panic(err)
		}
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

	Convey("before send message queue should be empty", t, func() {
		So(h.dht.Queue.Len(), ShouldEqual, 0)
	})

	Convey("after send message queue should have the message in it", t, func() {
		node, err := NewNode("/ip4/127.0.0.1/tcp/1234", h.Agent().PrivKey())
		if err != nil {
			panic(err)
		}
		defer node.Close()
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		r, err := h.dht.Send(node.HashAddr, PUT_REQUEST, hash)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
		So(h.dht.Queue.Len(), ShouldEqual, 1)
		messages, err := h.dht.Queue.Get(1)
		So(err, ShouldBeNil)
		m := messages[0].(*Message)
		So(m.Type, ShouldEqual, PUT_REQUEST)
		hs := m.Body.(Hash)
		So(hs.String(), ShouldEqual, "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	})
}
