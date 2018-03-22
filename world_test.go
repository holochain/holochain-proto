package holochain

import (
	"fmt"
	. "github.com/holochain/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func testAddNodeToWorld(world *World, ID peer.ID, addr ma.Multiaddr) {
	pi := pstore.PeerInfo{ID: ID, Addrs: []ma.Multiaddr{addr}}
	err := world.AddNode(pi)
	if err != nil {
		panic(err)
	}
}

func testAddNodesToWorld(world *World, start, count int) (nodes []*Node) {
	for i := start; i < start+count; i++ {
		node, err := makeNode(1234, fmt.Sprintf("node%d", i))
		if err != nil {
			panic(err)
		}
		nodes = append(nodes, node)
		testAddNodeToWorld(world, node.HashAddr, node.NetAddr)
	}
	return
}

func TestWorldNodes(t *testing.T) {
	b58 := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2"
	peer, _ := peer.IDB58Decode(b58)
	world := NewWorld(peer)

	Convey("to start with I should know about nobody", t, func() {
		nodes, err := world.AllNodes()
		So(err, ShouldBeNil)
		So(nodes, ShouldBeEmpty)
	})

	Convey("nodes can be added to the world model", t, func() {
		n := testAddNodesToWorld(world, 0, 1)
		nodes, err := world.AllNodes()
		So(err, ShouldBeNil)
		So(nodes[0], ShouldEqual, n[0].HashAddr)
	})

	Convey("nodes can be sorted by closeness to a hash", t, func() {
		testAddNodesToWorld(world, 1, 5)
		hash, _ := NewHash("QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHw")
		nodes, err := world.NodesByHash(hash)
		So(err, ShouldBeNil)
		So(len(nodes), ShouldEqual, 7) // 7 because NodesByHash should add in "me" too
		So(distance(nodes[0], hash).Cmp(distance(nodes[1], hash)), ShouldBeLessThanOrEqualTo, 0)
		So(distance(nodes[1], hash).Cmp(distance(nodes[2], hash)), ShouldBeLessThanOrEqualTo, 0)
		So(distance(nodes[2], hash).Cmp(distance(nodes[3], hash)), ShouldBeLessThanOrEqualTo, 0)
		So(distance(nodes[3], hash).Cmp(distance(nodes[4], hash)), ShouldBeLessThanOrEqualTo, 0)
		So(distance(nodes[4], hash).Cmp(distance(nodes[5], hash)), ShouldBeLessThanOrEqualTo, 0)
		So(distance(nodes[6], hash).Cmp(distance(nodes[6], hash)), ShouldBeLessThanOrEqualTo, 0)
	})

}

func TestWorldUpdateResponsible(t *testing.T) {
	b58 := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2"
	var p1, p2, p3, p4, p5 peer.ID
	var hash1, hash2, hash4 Hash
	p1, _ = peer.IDB58Decode(b58)
	world := NewWorld(p1)
	var addr ma.Multiaddr
	var err error
	var responsible bool
	testAddNodeToWorld(world, p1, addr)
	Convey("you should always be responsible for yourself!", t, func() {
		hash1 = HashFromPeerID(p1)
		responsible, err := world.UpdateResponsible(hash1, 0)
		So(err, ShouldBeNil)
		So(responsible, ShouldBeTrue)
		responsible, err = world.UpdateResponsible(hash1, 2)
		So(err, ShouldBeNil)
		So(responsible, ShouldBeTrue)
	})
	Convey("you shouldn't be responsible for far away hashes", t, func() {
		p2, _ = peer.IDB58Decode("QmY9Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		testAddNodeToWorld(world, p2, addr)
		p3, _ = peer.IDB58Decode("QmY9Mzg9F69e5P9AoQPYbt655HEhc1TVGs11tmfNSzkqh2")
		testAddNodeToWorld(world, p3, addr)
		p4, _ = peer.IDB58Decode("QmY8Mzg9F69e5P9AoQPYbt655HEhc1TVGs11tmfNSykqh2")
		testAddNodeToWorld(world, p4, addr)

		hash2 = HashFromPeerID(p2)
		responsible, err := world.UpdateResponsible(hash2, 2)
		So(err, ShouldBeNil)
		So(responsible, ShouldBeFalse)

		hash4 = HashFromPeerID(p4)
		responsible, err = world.UpdateResponsible(hash4, 2)
		So(err, ShouldBeNil)
		So(responsible, ShouldBeTrue)
	})
	Convey("when new closer nodes are added you should stop being responsible", t, func() {
		p5, _ = peer.IDB58Decode("QmY8Mzg9F69e5P9AoQPYbt655HEhc1TVGs11tmfNSykqh1")
		testAddNodeToWorld(world, p5, addr)

		hash4 = HashFromPeerID(p4)
		responsible, err = world.UpdateResponsible(hash4, 2)
		So(err, ShouldBeNil)
		So(responsible, ShouldBeFalse)
	})
}

func TestWorldOverlap(t *testing.T) {
	nodesCount := 20
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes
	h := nodes[0]

	starConnectMutual(t, mt.ctx, nodes, nodesCount)

	Convey("it should add all nodes to the world model", t, func() {
		nodes, err := h.world.AllNodes()
		So(err, ShouldBeNil)
		So(len(nodes), ShouldEqual, nodesCount-1)
	})

	Convey("when redundancy is 0 overlap is 100%", t, func() {
		for i := 0; i < nodesCount; i++ {
			chain := nodes[i].Chain()
			for _, hd := range chain.Headers {
				responsible, err := h.world.UpdateResponsible(hd.EntryLink, 0)
				So(err, ShouldBeNil)
				So(responsible, ShouldBeTrue)
			}
		}

		entries, err := h.world.Responsible()
		So(err, ShouldBeNil)
		So(len(entries), ShouldEqual, nodesCount+1) // all hashes are agent entries plus the DHA hash

		for i := 0; i < nodesCount; i++ {
			overlap, err := h.Overlap(nodes[i].AgentHash())
			So(err, ShouldBeNil)
			So(len(overlap), ShouldEqual, nodesCount-1) //all the nodes except me
		}
	})

	Convey("when redundancy is 5, and assuming no uptime adjustment, overlap should be 4 nodes", t, func() {
		r := 5
		for i := 0; i < nodesCount; i++ {
			nodes[i].nucleus.dna.DHTConfig.RedundancyFactor = r
			chain := nodes[i].Chain()
			for _, hd := range chain.Headers {
				h.world.UpdateResponsible(hd.EntryLink, r)
			}
		}

		entries, err := h.world.Responsible()
		So(err, ShouldBeNil)
		So(len(entries), ShouldEqual, 2) // I'm only responsible for some of the entries

		// for all entries there should be 4 other nodes that I hold responsible for it.
		for i := 0; i < len(entries); i++ {
			overlap, err := h.Overlap(entries[i])
			So(err, ShouldBeNil)
			So(len(overlap), ShouldEqual, 4)
		}
	})
}
