package holochain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	. "github.com/metacurrency/holochain/hash"
	ma "github.com/multiformats/go-multiaddr"
	. "github.com/smartystreets/goconvey/convey"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNodeMDNSDiscovery(t *testing.T) {
	nodesCount := 4
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes

	node0 := nodes[0].node
	node1 := nodes[1].node
	node2 := nodes[2].node
	node3 := nodes[3].node

	Convey("nodes should find eachother via mdns", t, func() {
		So(len(node0.host.Peerstore().Peers()), ShouldEqual, 1)
		So(len(node1.host.Peerstore().Peers()), ShouldEqual, 1)
		So(len(node2.host.Peerstore().Peers()), ShouldEqual, 1)

		err := node0.EnableMDNSDiscovery(nodes[0], time.Second/4)
		So(err, ShouldBeNil)
		err = node1.EnableMDNSDiscovery(nodes[1], time.Second/4)
		So(err, ShouldBeNil)
		err = node2.EnableMDNSDiscovery(nodes[2], time.Second/4)
		So(err, ShouldBeNil)

		time.Sleep(time.Millisecond * 500)

		So(len(node0.host.Peerstore().Peers()), ShouldEqual, 3)
		So(len(node1.host.Peerstore().Peers()), ShouldEqual, 3)
		So(len(node2.host.Peerstore().Peers()), ShouldEqual, 3)
	})

	Convey("nodes should confirm connectability before adding nodes found via mdns", t, func() {
		So(len(node3.host.Peerstore().Peers()), ShouldEqual, 1)

		// shut-down node2 so the new node can't connect to it
		node2.Close()

		err := node3.EnableMDNSDiscovery(nodes[3], time.Second/4)
		So(err, ShouldBeNil)
		time.Sleep(time.Millisecond * 500)

		/*for _, p := range node3.host.Peerstore().Peers() {
			fmt.Printf("PERR:%v\n", p)
			for _, a := range node3.host.Peerstore().Addrs(p) {
				fmt.Printf("    ADDR:%v\n", a)
			}
		}*/

		// mdns still reporting nodes2
		So(len(node3.host.Peerstore().Peers()), ShouldEqual, 4)
		//	So(len(node3.host.Peerstore().Addrs(nodes[2].nodeID)), ShouldEqual, 0)
	})
}

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

		node.host.Peerstore().AddAddr(node2.HashAddr, node2.NetAddr, pstore.PermanentAddrTTL)
		var payload string
		node2.host.SetStreamHandler("/testprotocol/1.0.0", func(s net.Stream) {
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

		s, err := node.host.NewStream(node.ctx, node2.HashAddr, "/testprotocol/1.0.0")
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
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	node1, err := makeNode(1234, "node1")
	if err != nil {
		panic(err)
	}
	h.node.Close()
	h.node = node1

	d2, _, h2 := PrepareTestChain("test2")
	defer CleanupTestChain(h2, d2)
	h2.node.Close()

	node2, err := makeNode(1235, "node2")
	if err != nil {
		panic(err)
	}
	defer node2.Close()
	h2.node = node2
	os.Remove(filepath.Join(h2.DBPath(), DHTStoreFileName))
	h2.dht = NewDHT(h2)

	h.Activate()

	Convey("It should start the DHT protocols", t, func() {
		err := h2.dht.Start()
		So(err, ShouldBeNil)
	})
	Convey("It should start the Nucleus protocols", t, func() {
		err := h2.nucleus.Start()
		So(err, ShouldBeNil)
	})

	node2.host.Peerstore().AddAddr(node1.HashAddr, node1.NetAddr, pstore.PermanentAddrTTL)

	Convey("It should fail on messages without a source", t, func() {
		m := Message{Type: PUT_REQUEST, Body: "fish"}
		So(len(node1.host.Peerstore().Peers()), ShouldEqual, 1)
		r, err := node2.Send(context.Background(), ActionProtocol, node1.HashAddr, &m)
		So(err, ShouldBeNil)
		So(len(node1.host.Peerstore().Peers()), ShouldEqual, 2) // node1's peerstore should now have node2
		So(r.Type, ShouldEqual, ERROR_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(r.Body.(ErrorResponse).Message, ShouldEqual, "message must have a source")
	})

	Convey("It should fail on incorrect message types", t, func() {
		m := node1.NewMessage(PUT_REQUEST, "fish")
		r, err := node1.Send(context.Background(), ValidateProtocol, node2.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, ERROR_RESPONSE)
		So(r.From, ShouldEqual, node2.HashAddr) // response comes from who we sent to
		So(r.Body.(ErrorResponse).Message, ShouldEqual, "message type 2 not in holochain-validate protocol")

		m = node2.NewMessage(PUT_REQUEST, "fish")
		r, err = node2.Send(context.Background(), GossipProtocol, node1.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, ERROR_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(r.Body.(ErrorResponse).Message, ShouldEqual, "message type 2 not in holochain-gossip protocol")

		m = node2.NewMessage(GOSSIP_REQUEST, "fish")
		r, err = node2.Send(context.Background(), ActionProtocol, node1.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, ERROR_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(r.Body.(ErrorResponse).Message, ShouldEqual, "message type 9 not in holochain-action protocol")

	})

	Convey("It should respond with err on bad request on invalid PUT_REQUESTS", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")

		m := node2.NewMessage(PUT_REQUEST, PutReq{H: hash})
		r, err := node2.Send(context.Background(), ActionProtocol, node1.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, ERROR_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(r.Body.(ErrorResponse).Code, ShouldEqual, ErrHashNotFoundCode)
	})

	Convey("It should respond with OK if valid request", t, func() {
		m := node2.NewMessage(GOSSIP_REQUEST, GossipReq{})
		r, err := node2.Send(context.Background(), GossipProtocol, node1.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, OK_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(fmt.Sprintf("%T", r.Body), ShouldEqual, "holochain.Gossip")
	})

	Convey("it should respond with err on messages from nodes on the blockedlist", t, func() {
		node1.Block(node2.HashAddr)
		m := node2.NewMessage(GOSSIP_REQUEST, GossipReq{})
		r, err := node2.Send(context.Background(), GossipProtocol, node1.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, ERROR_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(r.Body.(ErrorResponse).Code, ShouldEqual, ErrBlockedListedCode)
	})

	Convey("it should respond with err on messages to nodes on the blockedlist", t, func() {
		node1.Block(node2.HashAddr)
		m := node1.NewMessage(GOSSIP_REQUEST, GossipReq{})
		_, err = node1.Send(context.Background(), GossipProtocol, node2.HashAddr, m)
		So(err, ShouldEqual, ErrBlockedListed)
	})

}

func TestNodeBlockedList(t *testing.T) {
	Convey("it should be set up from a peerlist", t, func() {
		node, _ := makeNode(1234, "node1")
		defer node.Close()
		node2, _ := makeNode(1235, "node2")
		defer node2.Close()

		So(node.IsBlocked(node2.HashAddr), ShouldBeFalse)
		node.InitBlockedList(PeerList{Records: []PeerRecord{PeerRecord{ID: node2.HashAddr}}})
		So(node.IsBlocked(node2.HashAddr), ShouldBeTrue)
		node.Unblock(node2.HashAddr)
		So(node.IsBlocked(node2.HashAddr), ShouldBeFalse)
		node.Block(node2.HashAddr)
		So(node.IsBlocked(node2.HashAddr), ShouldBeTrue)
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
	Convey("It should encode and decode put messages", t, func() {
		d, err = m.Encode()
		So(err, ShouldBeNil)

		var m2 Message
		r := bytes.NewReader(d)
		err = m2.Decode(r)
		So(err, ShouldBeNil)

		So(fmt.Sprintf("%v", m), ShouldEqual, fmt.Sprintf("%v", &m2))
	})

	m = node.NewMessage(GET_REQUEST, "foo")
	Convey("It should encode and decode get messages", t, func() {
		d, err = m.Encode()
		So(err, ShouldBeNil)

		var m2 Message
		r := bytes.NewReader(d)
		err = m2.Decode(r)
		So(err, ShouldBeNil)

		So(fmt.Sprintf("%v", m), ShouldEqual, fmt.Sprintf("%v", &m2))
	})

	Convey("It should encode and decode get OK_RESPONSE", t, func() {
		body := GetResp{}
		body.Entry = GobEntry{C: "3"}

		m = node.NewMessage(OK_RESPONSE, body)
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
		So(f.String(), ShouldEqual, "Qmd7v7bxE7xRCj3Amhx8kyj7DbUGJdbKzuiUUahx3ARPec")
	})
}

func TestErrorCoding(t *testing.T) {
	Convey("it should encode and decode errors", t, func() {
		er := NewErrorResponse(ErrHashNotFound)
		So(er.DecodeResponseError(), ShouldEqual, ErrHashNotFound)
		er = NewErrorResponse(ErrHashDeleted)
		So(er.DecodeResponseError(), ShouldEqual, ErrHashDeleted)
		er = NewErrorResponse(ErrHashModified)
		So(er.DecodeResponseError(), ShouldEqual, ErrHashModified)
		er = NewErrorResponse(ErrHashRejected)
		So(er.DecodeResponseError(), ShouldEqual, ErrHashRejected)
		er = NewErrorResponse(ErrLinkNotFound)
		So(er.DecodeResponseError(), ShouldEqual, ErrLinkNotFound)

		er = NewErrorResponse(errors.New("Some Error"))
		So(er.Code, ShouldEqual, ErrUnknownCode)
		So(er.DecodeResponseError().Error(), ShouldEqual, "Some Error")
	})
}

func TestAddPeer(t *testing.T) {
	nodesCount := 4
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes

	h := nodes[0]
	somePeer := nodes[1].node.HashAddr
	pi := pstore.PeerInfo{ID: somePeer, Addrs: []ma.Multiaddr{nodes[1].node.NetAddr}}
	Convey("it should add a peer to the peer store and the gossip list", t, func() {
		So(h.node.routingTable.Size(), ShouldEqual, 0)
		So(len(h.node.peerstore.Peers()), ShouldEqual, 1)
		err := h.AddPeer(pi)
		So(err, ShouldBeNil)
		So(len(h.node.peerstore.Peers()), ShouldEqual, 2)
		glist, err := h.dht.getGossipers()
		So(err, ShouldBeNil)
		So(len(glist), ShouldEqual, 1)
		So(glist[0], ShouldEqual, somePeer)
		So(h.node.routingTable.Size(), ShouldEqual, 1)
	})

	Convey("it should not add a blocked peer", t, func() {
		blockedPeer := nodes[2].node.HashAddr
		bpi := pstore.PeerInfo{ID: blockedPeer, Addrs: []ma.Multiaddr{nodes[2].node.NetAddr}}
		h.node.Block(blockedPeer)
		err := h.AddPeer(bpi)
		So(err, ShouldEqual, ErrBlockedListed)
		So(len(h.node.peerstore.Peers()), ShouldEqual, 2)
		glist, err := h.dht.getGossipers()
		So(err, ShouldBeNil)
		So(len(glist), ShouldEqual, 1)
		So(glist[0], ShouldEqual, somePeer)
		So(h.node.routingTable.Size(), ShouldEqual, 1)
	})

	Convey("it should clear the peer's Address list if connection fails", t, func() {
		closedNode := nodes[nodesCount-1].node
		pi := pstore.PeerInfo{ID: closedNode.HashAddr, Addrs: []ma.Multiaddr{closedNode.NetAddr}}
		closedNode.Close()
		err := h.AddPeer(pi)
		So(err, ShouldEqual, nil)
		So(len(h.node.peerstore.Addrs(pi.ID)), ShouldEqual, 0)
	})
}

func TestNodeRouting(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	node := h.node

	start := 0
	testPeerCount := 20
	peers := []peer.ID{}

	peers = addTestPeers(h, peers, start, testPeerCount)
	Convey("populating routing", t, func() {
		p := node.HashAddr
		srch := node.routingTable.NearestPeers(HashFromPeerID(p), 5)
		nearest := fmt.Sprintf("%d %v", len(srch), srch)
		So(nearest, ShouldEqual, "5 [<peer.ID P9vKpw> <peer.ID P9QXqa> <peer.ID PrUBh5> <peer.ID Pn94bj> <peer.ID QHFWTH>]")
		start = testPeerCount
		testPeerCount += 5
		peers = addTestPeers(h, peers, start, testPeerCount)
		srch = node.routingTable.NearestPeers(HashFromPeerID(p), 5)
		nearest = fmt.Sprintf("%d %v", len(srch), srch)

		// adding a few more yields one which is closer
		So(nearest, ShouldEqual, "5 [<peer.ID NSQqJR> <peer.ID P9vKpw> <peer.ID P9QXqa> <peer.ID PrUBh5> <peer.ID Pn94bj>]")
		//		node.routingTable.Print()
	})
}

func TestNodeAppSendResolution(t *testing.T) {
	nodesCount := 50
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	ringConnect(t, mt.ctx, mt.nodes, nodesCount)
	node1 := mt.nodes[0].node
	node2 := mt.nodes[nodesCount/2].node

	Convey("sending to nodes we aren't directly connected to should resolve", t, func() {
		m := node2.NewMessage(GOSSIP_REQUEST, GossipReq{})
		r, err := node2.Send(mt.ctx, GossipProtocol, node1.HashAddr, m)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, OK_RESPONSE)
		So(r.From, ShouldEqual, node1.HashAddr) // response comes from who we sent to
		So(fmt.Sprintf("%T", r.Body), ShouldEqual, "holochain.Gossip")
	})
}

func TestActivePeers(t *testing.T) {
	node, _ := makeNode(1234, "")
	defer node.Close()
	Convey("self node should be active", t, func() {
		So(node.isPeerActive(node.HashAddr), ShouldBeTrue)
	})

	otherPeer, _ := makePeer("foo")
	Convey("a peer not in the peerstore be considered inactive", t, func() {
		So(node.isPeerActive(otherPeer), ShouldBeFalse)
	})

	Convey("a peer without an address in the peerstore should be considered inactive", t, func() {
		node.peerstore.AddAddrs(otherPeer, []ma.Multiaddr{}, PeerTTL)
		So(node.isPeerActive(otherPeer), ShouldBeFalse)
	})

	Convey("it should be able to filter out inactive peers from a list", t, func() {
		filteredList := node.filterInactviePeers([]peer.ID{node.HashAddr, otherPeer}, 0)
		So(len(filteredList), ShouldEqual, 1)
		So(filteredList[0], ShouldEqual, node.HashAddr)
	})

	Convey("it should be able to limit number returned when filtering out inactive peers", t, func() {
		addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/1234")
		if err != nil {
			panic(err)
		}
		node.peerstore.AddAddrs(otherPeer, []ma.Multiaddr{addr}, PeerTTL)
		filteredList := node.filterInactviePeers([]peer.ID{node.HashAddr, otherPeer}, 0)
		So(len(filteredList), ShouldEqual, 2)
		So(filteredList[0], ShouldEqual, node.HashAddr)
		So(filteredList[1], ShouldEqual, otherPeer)

		filteredList = node.filterInactviePeers([]peer.ID{node.HashAddr, otherPeer}, 1)
		So(len(filteredList), ShouldEqual, 1)
	})
}

func TestNodeStress(t *testing.T) {
	nodesCount := 4
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes
	h1 := nodes[0]
	h2 := nodes[1]
	h3 := nodes[2]
	h4 := nodes[3]
	node1 := h1.node
	node2 := h2.node
	node3 := h3.node
	node4 := h4.node
	starConnectMutual(t, mt.ctx, nodes, nodesCount)

	Convey("hammering a node should work", t, func() {
		var i int
		var err error
		var r Message
		//count := 1000
		count := 10
		s1 := make(chan bool, count)
		s2 := make(chan bool, count)
		for i = 0; i < count; i++ {
			hash := commit(h1, "evenNumbers", fmt.Sprintf("%d", i*2))
			m := node1.NewMessage(PUT_REQUEST, PutReq{H: hash})
			r, err = node1.Send(mt.ctx, ActionProtocol, node2.HashAddr, m)
			if err != nil || r.Type != OK_RESPONSE {
				break
			}
			go func() {
				r, err = node1.Send(mt.ctx, ActionProtocol, node3.HashAddr, m)
				if err != nil {
					panic(err)
				}
				s1 <- true
			}()
			go func() {
				r, err = node1.Send(mt.ctx, ActionProtocol, node4.HashAddr, m)
				if err != nil {
					panic(err)
				}
				s2 <- true
			}()
		}
		for i = 0; i < count; i++ {
			<-s1
			<-s2
		}
		So(i, ShouldEqual, count)
		So(err, ShouldBeNil)
		So(r.Type, ShouldEqual, OK_RESPONSE)
	})
}

func TestNodeRoutingTableBootstrap(t *testing.T) {
	nodesCount := 10
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes

	rt := nodes[0].node.routingTable
	Convey("it should should bootstrap the routing table", t, func() {
		So(rt.Size(), ShouldEqual, 0)
		So(rt.IsEmpty(), ShouldBeTrue)

		// connect up all the nodes except 0
		for i := 0; i < nodesCount-1; i++ {
			connect(t, mt.ctx, nodes[i], nodes[i+1])
			nodes[i].node.routingTable.Update(nodes[i+1].nodeID)
		}

		//now call routing refresh to boostrap the table
		RoutingRefreshTask(nodes[0])
		So(rt.Size(), ShouldEqual, 9)
		So(rt.IsEmpty(), ShouldBeFalse)
	})
}

// -------------------------------------------------------------------------------------------
// node testing functions

func makePeer(id string) (pid peer.ID, key ic.PrivKey) {
	// use a constant reader so the key will be the same each time for the test...
	var err error
	key, _, err = ic.GenerateEd25519Key(MakeTestSeed(id))
	if err != nil {
		panic(err)
	}
	pid, _ = peer.IDFromPrivateKey(key)
	return
}

func makeNode(port int, id string) (*Node, error) {
	listenaddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)
	_, key := makePeer(id)
	agent := LibP2PAgent{identity: AgentIdentity(id), priv: key, pub: key.GetPublic()}
	return NewNode(listenaddr, "fakednahash", &agent, false, &debugLog)
}

func addTestPeers(h *Holochain, peers []peer.ID, start int, count int) []peer.ID {
	for i := start; i < count; i++ {
		p, _ := makePeer(fmt.Sprintf("peer_%d", i))
		//		fmt.Printf("Peer %d: %s\n", i, peer.IDB58Encode(p))
		peers = append(peers, p)
		pi := pstore.PeerInfo{ID: p}
		err := h.addPeer(pi, false)
		if err != nil {
			panic(err)
		}
	}
	return peers
}

func ringConnect(t *testing.T, ctx context.Context, nodes []*Holochain, nodesCount int) {
	for i := 0; i < nodesCount; i++ {
		connect(t, ctx, nodes[i], nodes[(i+1)%len(nodes)])
	}
}

func ringConnectMutual(t *testing.T, ctx context.Context, nodes []*Holochain, nodesCount int) {
	for i := 0; i < nodesCount; i++ {
		n2 := nodes[(i+1)%len(nodes)]
		connect(t, ctx, nodes[i], n2)
		connect(t, ctx, n2, nodes[i])
	}
}

func starConnect(t *testing.T, ctx context.Context, nodes []*Holochain, nodesCount int) {
	for i := 1; i < nodesCount; i++ {
		connect(t, ctx, nodes[0], nodes[i])
	}
}

func starConnectMutual(t *testing.T, ctx context.Context, nodes []*Holochain, nodesCount int) {
	for i := 1; i < nodesCount; i++ {
		connect(t, ctx, nodes[0], nodes[i])
		connect(t, ctx, nodes[i], nodes[0])
	}
}

func randConnect(t *testing.T, ctx context.Context, nodes []*Holochain, nodesCount, connectFromCount, connectToCount int) {

	// connect nodes[1->connectFromCount] to connectToCount randomly selected nodes in
	// nodes[(nodesCount-connectFromCount)->randConnect]

	mrand := rand.New(rand.NewSource(42))
	guy := nodes[0]
	others := nodes[1:]
	for i := 0; i < connectFromCount; i++ {
		for j := 0; j < connectToCount; j++ { // 16, high enough to probably not have any partitions
			v := mrand.Intn(nodesCount - connectFromCount - 1)
			connect(t, ctx, others[i], others[connectFromCount+v])
		}
	}

	for i := 0; i < connectFromCount; i++ {
		connect(t, ctx, guy, others[i])
	}
}

type multiNodeTest struct {
	ctx    context.Context
	cancel context.CancelFunc
	s      *Service
	d      string
	nodes  []*Holochain
	count  int
}

func setupMultiNodeTesting(n int) (mt *multiNodeTest) {
	ctx, cancel := context.WithCancel(context.Background())
	d, s := SetupTestService()
	mt = &multiNodeTest{
		ctx:    ctx,
		cancel: cancel,
		s:      s,
		d:      d,
		count:  n,
	}
	mt.nodes = makeTestNodes(mt.ctx, mt.s, n)
	return
}

func (mt *multiNodeTest) cleanupMultiNodeTesting() {
	for i := 0; i < mt.count; i++ {
		mt.nodes[i].Close()
	}
	mt.cancel()
	CleanupTestDir(mt.d)
}

func makeTestNodes(ctx context.Context, s *Service, n int) (nodes []*Holochain) {
	nodes = make([]*Holochain, n)
	for i := 0; i < n; i++ {
		nodeName := fmt.Sprintf("node%d", i)
		os.Setenv("HCLOG_PREFIX", nodeName+"_")
		nodes[i] = setupTestChain(nodeName, i, s)
		nodes[i].Config.EnableMDNS = false
		prepareTestChain(nodes[i])
	}
	for i := 0; i < n; i++ {
		nodeName := fmt.Sprintf("node%d", i)
		nodes[i].dht.dlog.Logf("SETUP: %s is %v", nodeName, nodes[i].nodeID)
	}
	os.Unsetenv("HCLOG_PREFIX")
	return
}

func connectNoSync(t *testing.T, ctx context.Context, ah, bh *Holochain) {
	a := ah.node
	b := bh.node
	idB := b.HashAddr
	addrB := b.peerstore.Addrs(idB)
	if len(addrB) == 0 {
		t.Fatal("peers setup incorrectly: no local address")
	}

	pi := pstore.PeerInfo{ID: idB, Addrs: addrB}
	err := ah.AddPeer(pi)
	if err != nil {
		t.Fatal(err)
	}

	if err = a.host.Connect(ctx, pi); err != nil {
		t.Fatal(err)
	}
}

func connect(t *testing.T, ctx context.Context, a, b *Holochain) {
	connectNoSync(t, ctx, a, b)

	// loop until connection notification has been received.
	// under high load, this may not happen as immediately as we would like.
	/*	for a.node.routingTable.Find(b.nodeID) == "" {
			time.Sleep(time.Millisecond * 5)
		}

		for b.node.routingTable.Find(a.nodeID) == "" {
			time.Sleep(time.Millisecond * 5)
		}*/
	//	time.Sleep(100 * time.Millisecond)
}
