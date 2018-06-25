package holochain

import (
	"fmt"
	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	_ "time"
)

func TestGetClosestPeers(t *testing.T) {
	nodesCount := 30
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes

	randConnect(t, mt.ctx, nodes, nodesCount, 7, 4)

	Convey("it should return a list of close peers", t, func() {
		fooHash, _ := NewHash("QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx")
		//fooHash := HashFromPeerID(nodes[29].node.HashAddr)
		peers, err := nodes[1].node.GetClosestPeers(mt.ctx, fooHash)
		So(err, ShouldBeNil)

		var out []peer.ID
		for p := range peers {
			out = append(out, p)
		}

		So(len(out), ShouldEqual, KValue)
	})

	starConnect(t, mt.ctx, nodes, nodesCount)

	Convey("nodes should agree on who's the closest to a hash", t, func() {
		hash, _ := NewHash("QmS4bKx7zZt6qoX2om5M5ik3X2k4Fco2nFx82CDJ3iVKj2")
		var closest peer.ID
		for i, h := range nodes {
			peers, err := h.node.GetClosestPeers(mt.ctx, hash)
			if err != nil {
				fmt.Printf("%d--%v:  %v", i, h.nodeID.Pretty(), err)
			} else {
				var out []peer.ID
				for p := range peers {
					out = append(out, p)
				}
				//fmt.Printf("%v thinks %v,%v is closest\n", h.nodeID.Pretty()[2:4], out[0].Pretty()[2:4], out[1].Pretty()[2:4])

				if i != 0 {
					So(closest.Pretty(), ShouldEqual, out[0].Pretty())
				} else {
					closest = out[0]
				}
			}
		}
	})

}
