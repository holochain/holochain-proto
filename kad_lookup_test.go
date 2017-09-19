package holochain

import (
	"context"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

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

	SkipConvey("it should return a list of close peers", t, func() {
		for i := 0; i < nodesCount; i++ {
			connect(t, ctx, nodes[i].node, nodes[(i+1)%len(nodes)].node)
		}

		peers, err := nodes[0].node.GetClosestPeers(ctx, nodes[0].node.HashAddr)
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
	if err := a.Host.Connect(ctx, pi); err != nil {
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
