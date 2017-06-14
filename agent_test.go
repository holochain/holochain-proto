package holochain

import (
	"bytes"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
)

func TestLibP2PAgent(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
	a := AgentName("zippy@someemail.com")

	Convey("it should fail to create an agent with an unknown key type", t, func() {
		_, err := NewAgent(99, a)
		So(err.Error(), ShouldEqual, "unknown key type: 99")
	})
	Convey("it should be create a new agent that is saved to a file and then loadable", t, func() {
		a1, err := NewAgent(LibP2P, a)
		So(err, ShouldBeNil)
		err = SaveAgent(d, a1)
		So(err, ShouldBeNil)
		a2, err := LoadAgent(d)
		So(err, ShouldBeNil)
		So(a2.Name(), ShouldEqual, a1.Name())
		So(ic.KeyEqual(a1.PrivKey(), a2.PrivKey()), ShouldBeTrue)
		So(a1.KeyType(), ShouldEqual, LibP2P)

		nodeID, nodeIDStr, err := a1.NodeID()
		So(nodeIDStr, ShouldEqual, peer.IDB58Encode(nodeID))
		So(nodeID.MatchesPublicKey(a1.PubKey()), ShouldBeTrue)

	})
	Convey("it should fail to load an agent file that has bad permissions", t, func() {
		os.Chmod(d+"/"+PrivKeyFileName, OS_USER_RW)
		_, err := LoadAgent(d)
		So(err.Error(), ShouldEqual, d+"/"+PrivKeyFileName+" file not read-only")
	})
	Convey("genkeys with with nil reader should use random seed", t, func() {
		agent, _ := NewAgent(LibP2P, a)
		_, n1, _ := agent.NodeID()
		agent.GenKeys(nil)
		_, n2, _ := agent.NodeID()
		So(n1, ShouldNotEqual, n2)
	})

	Convey("genkeys with fixed seed should generate the same key", t, func() {
		agent, err := NewAgent(LibP2P, a)
		So(err, ShouldBeNil)
		_, n1, err := agent.NodeID()
		So(err, ShouldBeNil)
		err = agent.GenKeys(bytes.NewBuffer([]byte("fixed seed 012345678901234567890123456789")))
		So(err, ShouldBeNil)
		_, n2, err := agent.NodeID()
		So(err, ShouldBeNil)
		So(n1, ShouldNotEqual, n2)
		err = agent.GenKeys(bytes.NewBuffer([]byte("fixed seed 012345678901234567890123456789")))
		So(err, ShouldBeNil)
		_, n1, _ = agent.NodeID()
		So(n1, ShouldEqual, n2)
		err = agent.GenKeys(bytes.NewBuffer([]byte("different seed012345678901234567890123456789")))
		So(err, ShouldBeNil)
		_, n2, _ = agent.NodeID()
		So(n1, ShouldNotEqual, n2)
	})
}
