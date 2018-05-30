package holochain

import (
	"context"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	"math/rand"
	"os"
	"testing"
)

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

func fullConnect(t *testing.T, ctx context.Context, nodes []*Holochain, nodesCount int) {
	for i := 0; i < nodesCount; i++ {
		for j := 0; j < nodesCount; j++ {
			if j != i {
				connect(t, ctx, nodes[i], nodes[j])
			}
		}
	}
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
