package holochain

import (
	_ "fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

/*
func TestPutGet(t *testing.T) {
	dht := Needn't()
	Convey("It should store and retrieve", t, func() {
		h := NewHash("1vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")
		err := dht.Put(h)
		So(err, ShouldBeNil)
		data, err := dht.Get(h)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "fake value")
		data, err = dht.Get(NewHash("2vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA"))
		So(err.Error(), ShouldEqual, "No key: 2vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")

	})
}*/

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
		So(len(h.dht.Queue), ShouldEqual, 0)
	})

	Convey("after send message queue should have the message in it", t, func() {
		node, err := NewNode("/ip4/127.0.0.1/tcp/1234", h.Agent().PrivKey())
		if err != nil {
			panic(err)
		}
		defer node.Close()
		r, err := h.dht.Send(node.HashAddr, PUT_REQUEST, "some message")
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
		/*		So(len(h.dht.Queue), ShouldEqual, 1)
				m := h.dht.Queue[0]
				So(m.Type, ShouldEqual, PUT_REQUEST)
				So(m.Body.(string), ShouldEqual, "some message")*/
	})
}
