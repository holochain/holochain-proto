package holochain

import (
	"context"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"math/rand"
	"testing"
	"time"
)

func ringConnect(t *testing.T, ctx context.Context, nodes []*Holochain, nodesCount int) {
	for i := 0; i < nodesCount; i++ {
		connect(t, ctx, nodes[i].node, nodes[(i+1)%len(nodes)].node)
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
			connect(t, ctx, others[i].node, others[connectFromCount+v].node)
		}
	}

	for i := 0; i < connectFromCount; i++ {
		connect(t, ctx, guy.node, others[i].node)
	}
}

func TestGetClosestPeers(t *testing.T) {
	d, s := SetupTestService()
	defer CleanupTestDir(d)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodesCount := 30
	nodes := makeTestNodes(ctx, s, nodesCount)
	defer func() {
		for i := 0; i < nodesCount; i++ {
			nodes[i].Close()
		}
	}()

	randConnect(t, ctx, nodes, nodesCount, 15, 8)

	Convey("it should return a list of close peers", t, func() {
		fooHash, _ := NewHash("QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx")
		//fooHash := HashFromPeerID(nodes[29].node.HashAddr)
		peers, err := nodes[1].node.GetClosestPeers(ctx, fooHash)
		So(err, ShouldBeNil)

		var out []peer.ID
		for p := range peers {
			out = append(out, p)
		}

		So(len(out), ShouldEqual, KValue)
	})
}

func makeTestNodes(ctx context.Context, s *Service, n int) (nodes []*Holochain) {
	nodes = make([]*Holochain, n)
	for i := 0; i < n; i++ {
		nodes[i] = setupTestChain(fmt.Sprintf("node%d", i), i, s)
		prepareTestChain(nodes[i])

	}
	return
}

func connectNoSync(t *testing.T, ctx context.Context, a, b *Node) {
	idB := b.HashAddr
	addrB := b.peerstore.Addrs(idB)
	if len(addrB) == 0 {
		t.Fatal("peers setup incorrectly: no local address")
	}

	a.peerstore.AddAddrs(idB, addrB, pstore.TempAddrTTL)
	pi := pstore.PeerInfo{ID: idB}
	if err := a.host.Connect(ctx, pi); err != nil {
		t.Fatal(err)
	}
}

func connect(t *testing.T, ctx context.Context, a, b *Node) {
	connectNoSync(t, ctx, a, b)

	// loop until connection notification has been received.
	// under high load, this may not happen as immediately as we would like.
	for a.routingTable.Find(b.HashAddr) == "" {
		time.Sleep(time.Millisecond * 5)
	}

	for b.routingTable.Find(a.HashAddr) == "" {
		time.Sleep(time.Millisecond * 5)
	}
}
