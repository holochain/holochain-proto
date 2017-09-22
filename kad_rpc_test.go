package holochain

import (
	"context"
	routing "github.com/libp2p/go-libp2p-routing"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestNodeFindLocal(t *testing.T) {
	Convey("it should return empty record if not in routing table", t, func() {
	})

	Convey("it should return peerinfo if in routing table", t, func() {
	})
}

func TestNodeFindPeerSingle(t *testing.T) {
	Convey("FIND_NODE_REQUEST should X", t, func() {
	})
}

func TestKademliaReceiver(t *testing.T) {
	d, _, _ := PrepareTestChain("test")
	defer CleanupTestDir(d)

	Convey("FIND_NODE_REQUEST should X", t, func() {
	})

}

func TestFindPeer(t *testing.T) {
	// t.Skip("skipping test to debug another")
	if testing.Short() {
		t.SkipNow()
	}
	d, s := SetupTestService()
	defer CleanupTestDir(d)

	ctx := context.Background()

	nodes := makeTestNodes(ctx, s, 4)
	defer func() {
		for i := 0; i < 4; i++ {
			nodes[i].Close()
		}
	}()

	ctxT, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	Convey("searching before connected should fail with empty routing table", t, func() {
		_, err := nodes[0].node.FindPeer(ctxT, nodes[2].node.HashAddr)
		So(err, ShouldEqual, ErrEmptyRoutingTable)
	})

	connect(t, ctx, nodes[0].node, nodes[1].node)
	connect(t, ctx, nodes[1].node, nodes[2].node)
	connect(t, ctx, nodes[1].node, nodes[3].node)

	Convey("searching for unreachable node should fail with node not found", t, func() {
		unknownPeer, _ := makePeer("unknown peer")
		_, err := nodes[0].node.FindPeer(ctxT, unknownPeer)
		So(err, ShouldEqual, routing.ErrNotFound)
	})

	Convey("searching for a node connected through another should succeed", t, func() {
		p, err := nodes[0].node.FindPeer(ctxT, nodes[2].node.HashAddr)
		So(err, ShouldBeNil)
		So(p.ID, ShouldEqual, nodes[2].node.HashAddr)
	})

}
