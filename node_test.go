package holochain

import (
	"bytes"
	"context"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	. "github.com/smartystreets/goconvey/convey"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
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

func TestNewMessage(t *testing.T) {
	node, err := makeNode(1234, "node1")
	if err != nil {
		panic(err)
	}
	defer node.Close()
	Convey("It should create a new message", t, func() {
		now := time.Now()
		m := node.NewMessage(PUT_REQUEST, "fish")
		So(now.Before(m.Time), ShouldBeTrue) // poor check, but at least makes sure the time was set to something just after the NewMessage call was made
		So(m.Type, ShouldEqual, PUT_REQUEST)
		So(m.Body, ShouldEqual, "fish")
		So(m.From, ShouldEqual, node.HashAddr)
	})
}

func TestNodeSend(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)

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
	h.rootPath = d
	h.node = node1
	os.MkdirAll(h.DBPath(), os.ModePerm)
	h.dht = NewDHT(&h)
	h.chain = NewChain()

	Convey("It should start the DHT protocol", t, func() {
		err := h.dht.StartDHT()
		So(err, ShouldBeNil)
	})
	Convey("It should start the Validate protocol", t, func() {
		err := node2.StartValidate(&h)
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
		m := node1.NewMessage(PUT_REQUEST, "fish")
		r, err := node1.Send(ValidateProtocol, node2.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, ERROR_RESPONSE)
		So(r.From, ShouldEqual, node2.HashAddr) // response comes from who we sent to
		So(r.Body, ShouldEqual, "message type 2 not in holochain-validate protocol")
	})

	Convey("It should respond with queued on valid PUT_REQUESTS", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")

		m := node2.NewMessage(PUT_REQUEST, PutReq{H: hash})
		r, err := node2.Send(DHTProtocol, node1.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, OK_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(r.Body, ShouldEqual, "queued")
	})

}

func TestMessageCoding(t *testing.T) {
	node, err := makeNode(1234, "node1")
	if err != nil {
		panic(err)
	}
	defer node.Close()

	m := node.NewMessage(PUT_REQUEST, "foo")
	var d []byte
	Convey("It should encode and decode messages", t, func() {
		d, err = m.Encode()
		So(err, ShouldBeNil)

		var m2 Message
		r := bytes.NewReader(d)
		err = m2.Decode(r)
		So(err, ShouldBeNil)

		So(fmt.Sprintf("%v", m), ShouldEqual, fmt.Sprintf("%v", &m2))
	})
}

func TestFingerprintMessage(t *testing.T) {
	Convey("it should create a unique fingerprint for messages", t, func() {
		var id peer.ID
		var mp *Message
		f, err := mp.Fingerprint()
		So(err, ShouldBeNil)
		So(f.String(), ShouldEqual, NullHash().String())
		now := time.Unix(1, 1) // pick a constant time so the test will always work
		m := Message{Type: PUT_REQUEST, Time: now, Body: "foo", From: id}
		f, err = m.Fingerprint()
		So(err, ShouldBeNil)
		So(f.String(), ShouldEqual, "QmTZf2qqYiKbJbQVpFyidMVyAtb1S4xQNV52LcX9LDVTQn")
		m = Message{Type: PUT_REQUEST, Time: now, Body: "foo1", From: id}
		f, err = m.Fingerprint()
		So(err, ShouldBeNil)
		So(f.String(), ShouldEqual, "QmP2WUSMWAuZrX2nqWcEyei7GDCwVaetkynQESFDrHNkGa")
		now = time.Unix(1, 2) // pick a constant time so the test will always work
		m = Message{Type: PUT_REQUEST, Time: now, Body: "foo", From: id}
		f, err = m.Fingerprint()
		So(err, ShouldBeNil)
		So(f.String(), ShouldEqual, "QmTZf2qqYiKbJbQVpFyidMVyAtb1S4xQNV52LcX9LDVTQn")
		m = Message{Type: GET_REQUEST, Time: now, Body: "foo", From: id}
		f, err = m.Fingerprint()
		So(err, ShouldBeNil)
		So(f.String(), ShouldEqual, "QmTFnFfP5jkvyQkPPv4H8yoWqmiU82JT9dcHZAR4Hba6uy")
	})
}

/*
func TestFindPeer(t *testing.T) {
	node1, err := makeNode(1234, "node1")
	if err != nil {
		panic(err)
	}
	defer node1.Close()

	// generate a new unknown peerID
	r := strings.NewReader("1234567890123456789012345678901234567890x")
	key, _, err := ic.GenerateEd25519Key(r)
	if err != nil {
		panic(err)
	}
	pid, err := peer.IDFromPrivateKey(key)
	if err != nil {
		panic(err)
	}

	Convey("sending to an unknown peer should fail with no route to peer", t, func() {
		m := Message{Type: PUT_REQUEST, Body: "fish"}
		_, err := node1.Send(DHTProtocol, pid, &m)
		//So(r, ShouldBeNil)
		So(err, ShouldEqual, "fish")
	})

}
*/

func makeNode(port int, id string) (*Node, error) {
	listenaddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)
	// use a constant reader so the key will be the same each time for the test...
	r := strings.NewReader(id + "1234567890123456789012345678901234567890")
	key, _, err := ic.GenerateEd25519Key(r)
	if err != nil {
		panic(err)
	}
	pid, _ := peer.IDFromPrivateKey(key)

	return NewNode(listenaddr, pid, key)
}
