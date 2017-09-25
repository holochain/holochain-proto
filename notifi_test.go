package holochain

import (
	"testing"
)

func TestNotifieeMultipleConn(t *testing.T) {
	nodesCount := 2
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes

	n1 := nodes[0].node
	n2 := nodes[1].node

	nn1 := (*netNotifiee)(n1)
	nn2 := (*netNotifiee)(n2)

	connect(t, mt.ctx, nodes[0], nodes[1])
	c12 := n1.host.Network().ConnsToPeer(n2.HashAddr)[0]
	c21 := n2.host.Network().ConnsToPeer(n1.HashAddr)[0]

	// Pretend to reestablish/re-kill connection
	nn1.Connected(n1.host.Network(), c12)
	nn2.Connected(n2.host.Network(), c21)

	if !checkRoutingTable(n1, n2) {
		t.Fatal("no routes")
	}
	nn1.Disconnected(n1.host.Network(), c12)
	nn2.Disconnected(n2.host.Network(), c21)

	if !checkRoutingTable(n1, n2) {
		t.Fatal("no routes")
	}

	for _, conn := range n1.host.Network().ConnsToPeer(n2.HashAddr) {
		conn.Close()
	}
	for _, conn := range n2.host.Network().ConnsToPeer(n1.HashAddr) {
		conn.Close()
	}

	if checkRoutingTable(n1, n2) {
		t.Fatal("routes")
	}
}

func TestNotifieeFuzz(t *testing.T) {
	nodesCount := 2
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes

	n1 := nodes[0].node
	n2 := nodes[1].node

	for i := 0; i < 100; i++ {
		connectNoSync(t, mt.ctx, nodes[0], nodes[1])
		for _, conn := range n1.host.Network().ConnsToPeer(n2.HashAddr) {
			conn.Close()
		}
	}
	if checkRoutingTable(n1, n2) {
		t.Fatal("should not have routes")
	}
	connect(t, mt.ctx, nodes[0], nodes[1])
}

func checkRoutingTable(a, b *Node) bool {
	// loop until connection notification has been received.
	// under high load, this may not happen as immediately as we would like.
	return a.routingTable.Find(b.HashAddr) != "" && b.routingTable.Find(a.HashAddr) != ""
}
