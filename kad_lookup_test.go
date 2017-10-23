package holochain

import (
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
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
}
