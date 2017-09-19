package holochain

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"
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
