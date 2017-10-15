package holochain

import (
	"context"
	"fmt"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	routing "github.com/libp2p/go-libp2p-routing"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestNodeFindLocal(t *testing.T) {
	nodesCount := 3
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes
	node0 := nodes[0].node
	node1 := nodes[1].node
	node2 := nodes[2].node
	Convey("it should return empty record if not in routing table", t, func() {
		pi := node0.FindLocal(node1.HashAddr)
		So(fmt.Sprintf("%v", pi), ShouldEqual, fmt.Sprintf("%v", pstore.PeerInfo{}))
	})

	Convey("it should return peerinfo if in peerstore", t, func() {
		connect(t, mt.ctx, nodes[0], nodes[1])
		pi := node0.FindLocal(node1.HashAddr)
		So(pi.ID.Pretty(), ShouldEqual, node1.HashAddr.Pretty())
	})

	Convey("it should return peerinfo if in Routing Table", t, func() {
		node0.routingTable.Update(node2.HashAddr)
		pi := node0.FindLocal(node2.HashAddr)
		So(pi.ID.Pretty(), ShouldEqual, node2.HashAddr.Pretty())
	})

}

func TestKademliaReceiver(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("FIND_NODE_REQUEST should X", t, func() {
	})

}

func TestNodeFindPeer(t *testing.T) {
	// t.Skip("skipping test to debug another")
	if testing.Short() {
		t.SkipNow()
	}

	nodesCount := 6
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes

	ctxT, cancel := context.WithTimeout(mt.ctx, time.Second*5)
	defer cancel()

	Convey("searching before connected should fail with empty routing table", t, func() {
		_, err := nodes[0].node.FindPeer(ctxT, nodes[2].node.HashAddr)
		So(err, ShouldEqual, ErrEmptyRoutingTable)
	})

	for i := 0; i < nodesCount-1; i++ {
		connect(t, mt.ctx, nodes[i], nodes[i+1])
	}

	Convey("searching for unreachable node should fail with node not found", t, func() {
		unknownPeer, _ := makePeer("unknown peer")
		_, err := nodes[0].node.FindPeer(ctxT, unknownPeer)
		So(err, ShouldEqual, routing.ErrNotFound)
	})

	lastNode := nodes[nodesCount-1].node.HashAddr
	Convey("searching for a node connected through others should succeed", t, func() {

		pi, err := nodes[0].node.FindPeer(ctxT, lastNode)
		So(err, ShouldBeNil)
		So(pi.ID, ShouldEqual, lastNode)
	})

	Convey("findPeerSingle should return closer peers", t, func() {
		c, err := nodes[0].node.findPeerSingle(mt.ctx, nodes[1].node.HashAddr, HashFromPeerID(lastNode))
		So(err, ShouldBeNil)
		So(len(c), ShouldEqual, 1)
		So(c[0].ID, ShouldEqual, nodes[2].node.HashAddr)
	})
}
