package holochain

import (
	"bytes"
	"context"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	. "github.com/smartystreets/goconvey/convey"
	"io/ioutil"
	"strings"
	"testing"
)

func TestNewNode(t *testing.T) {

	node, err := makeNode(1234, "")
	defer node.Close()
	Convey("It should create a node", t, func() {
		So(err, ShouldBeNil)
		So(node.NetAddr.String(), ShouldEqual, "/ip4/127.0.0.1/tcp/1234")
		So(node.HashAddr.Pretty(), ShouldEqual, "QmNN6oDiV4GsfKDXPVmGLdBLLXCM28Jnm7pz7WD63aiwSG")
	})

	Convey("It should send between nodes", t, func() {
		node2, err := makeNode(4321, "node2")
		So(err, ShouldBeNil)
		defer node2.Close()

		node.Host.Peerstore().AddAddr(node2.HashAddr, node2.NetAddr, pstore.PermanentAddrTTL)
		var payload string
		node2.Host.SetStreamHandler("/testprotocol/1.0.0", func(s net.Stream) {
			defer s.Close()

			buf := make([]byte, 1024)
			n, err := s.Read(buf)
			if err != nil {
				payload = err.Error()
			} else {
				payload = string(buf[:n])
			}

			_, err = s.Write([]byte("I got: " + payload))

			if err != nil {
				panic(err)
			}
		})

		s, err := node.Host.NewStream(context.Background(), node2.HashAddr, "/testprotocol/1.0.0")
		So(err, ShouldBeNil)
		_, err = s.Write([]byte("greetings"))
		So(err, ShouldBeNil)

		out, err := ioutil.ReadAll(s)
		So(err, ShouldBeNil)
		So(payload, ShouldEqual, "greetings")
		So(string(out), ShouldEqual, "I got: greetings")
	})
}

func TestNodeSend(t *testing.T) {

	node1, err := makeNode(1234, "node1")
	if err != nil {
		panic(err)
	}
	defer node1.Close()

	node2, err := makeNode(1235, "node2")
	if err != nil {
		panic(err)
	}
	defer node2.Close()

	var h Holochain
	h.node = node1
	h.dht = NewDHT(&h)
	Convey("It should start the DHT protocol", t, func() {
		err := h.dht.StartDHT()
		So(err, ShouldBeNil)
	})
	Convey("It should start the Src protocol", t, func() {
		err := node2.StartSrc(&h)
		So(err, ShouldBeNil)
	})

	node1.Host.Peerstore().AddAddr(node2.HashAddr, node2.NetAddr, pstore.PermanentAddrTTL)
	node2.Host.Peerstore().AddAddr(node1.HashAddr, node1.NetAddr, pstore.PermanentAddrTTL)

	Convey("It should fail on messages without a source", t, func() {
		m := Message{Type: PUT_REQUEST, Body: "fish"}
		r, err := node2.Send(DHTProtocol, node1.HashAddr, &m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, ERROR_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(r.Body, ShouldEqual, "message must have a source")
	})

	Convey("It should fail on incorrect message types", t, func() {
		m := node1.makeMessage(PUT_REQUEST, "fish")
		r, err := node1.Send(SourceProtocol, node2.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, ERROR_RESPONSE)
		So(r.From, ShouldEqual, node2.HashAddr) // response comes from who we sent to
		So(r.Body, ShouldEqual, "message type 2 not in holochain-src protocol")
	})

	Convey("It should respond with queued on valid PUT_REQUESTS", t, func() {
		m := node2.makeMessage(PUT_REQUEST, "fish")
		r, err := node2.Send(DHTProtocol, node1.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, OK_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(r.Body, ShouldEqual, "queued")
	})

}

func TestMessageCoding(t *testing.T) {
	m := Message{Type: PUT_REQUEST, Body: "fish"}
	var d []byte
	var err error
	Convey("It should encode messages", t, func() {
		d, err = m.Encode()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", d), ShouldEqual, "[58 255 139 3 1 1 7 77 101 115 115 97 103 101 1 255 140 0 1 4 1 4 84 121 112 101 1 4 0 1 4 84 105 109 101 1 255 132 0 1 4 70 114 111 109 1 12 0 1 4 66 111 100 121 1 16 0 0 0 16 255 131 5 1 1 4 84 105 109 101 1 255 132 0 0 0 21 255 140 1 4 3 6 115 116 114 105 110 103 12 6 0 4 102 105 115 104 0]")

	})
	Convey("It should decode messages", t, func() {
		var m2 Message
		r := bytes.NewReader(d)
		err = m2.Decode(r)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", m), ShouldEqual, "{2 0001-01-01 00:00:00 +0000 UTC <peer.ID > fish}")

	})
}

func makeNode(port int, id string) (*Node, error) {
	listenaddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)
	// use a constant reader so the key will be the same each time for the test...
	r := strings.NewReader(id + "1234567890123456789012345678901234567890")
	key, _, err := ic.GenerateEd25519Key(r)
	if err != nil {
		panic(err)
	}
	return NewNode(listenaddr, key)
}
