package holochain

import (
	"context"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"math/rand"
	"os"
	"testing"
	_ "time"
)

func ringConnect(t *testing.T, ctx context.Context, nodes []*Holochain, nodesCount int) {
	for i := 0; i < nodesCount; i++ {
		connect(t, ctx, nodes[i], nodes[(i+1)%len(nodes)])
	}
}

func starConnect(t *testing.T, ctx context.Context, nodes []*Holochain, nodesCount int) {
	for i := 1; i < nodesCount; i++ {
		connect(t, ctx, nodes[0], nodes[i])
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
	ah.AddPeer(pi)

	if err := a.host.Connect(ctx, pi); err != nil {
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
