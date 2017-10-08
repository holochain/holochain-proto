package holochain

import (
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"path/filepath"
	"testing"
)

type FakeRevocation struct {
	data string
}

func (r *FakeRevocation) Verify() (err error) {
	return
}
func (r *FakeRevocation) Marshal() (bytes []byte, err error) {
	bytes = []byte(r.data)
	return
}

func (r *FakeRevocation) Unmarshal(data []byte) (err error) {
	r.data = string(data)
	return
}

func TestLibP2PAgent(t *testing.T) {
	d := SetupTestDir()
	defer CleanupTestDir(d)
	a := AgentIdentity("zippy@someemail.com")

	Convey("it should fail to create an agent with an unknown key type", t, func() {
		_, err := NewAgent(99, a, MakeTestSeed(""))
		So(err.Error(), ShouldEqual, "unknown key type: 99")
	})
	Convey("it should be create a new agent that is saved to a file and then loadable", t, func() {
		a1, err := NewAgent(LibP2P, a, MakeTestSeed(""))
		So(err, ShouldBeNil)
		err = SaveAgent(d, a1)
		So(err, ShouldBeNil)
		a2, err := LoadAgent(d)
		So(err, ShouldBeNil)
		So(a2.Identity(), ShouldEqual, a1.Identity())
		So(ic.KeyEqual(a1.PrivKey(), a2.PrivKey()), ShouldBeTrue)
		So(a1.AgentType(), ShouldEqual, LibP2P)

		nodeID, nodeIDStr, err := a1.NodeID()
		So(nodeIDStr, ShouldEqual, peer.IDB58Encode(nodeID))
		So(nodeID.MatchesPublicKey(a1.PubKey()), ShouldBeTrue)
	})
	Convey("it should be able to create an AgentEntry for a chain", t, func() {
		a1, err := NewAgent(LibP2P, a, MakeTestSeed(""))
		So(err, ShouldBeNil)
		revocation := FakeRevocation{data: "fake revocation"}
		entry, err := a1.AgentEntry(&revocation)
		So(err, ShouldBeNil)
		So(entry.Identity, ShouldEqual, a)
		So(string(entry.Revocation), ShouldEqual, "fake revocation")
		pk, _ := ic.MarshalPublicKey(a1.PubKey())
		So(string(entry.PublicKey), ShouldEqual, string(pk))
	})
	Convey("it should fail to load an agent file that has bad permissions", t, func() {
		os.Chmod(filepath.Join(d, PrivKeyFileName), OS_USER_RW)
		_, err := LoadAgent(d)
		So(err.Error(), ShouldEqual, filepath.Join(d, PrivKeyFileName)+" file not read-only")
	})
	Convey("genkeys with with nil reader should use random seed", t, func() {
		agent, _ := NewAgent(LibP2P, a, MakeTestSeed(""))
		_, n1, _ := agent.NodeID()
		agent.GenKeys(nil)
		_, n2, _ := agent.NodeID()
		So(n1, ShouldNotEqual, n2)
	})
	Convey("genkeys with fixed seed should generate the same key", t, func() {
		agent, err := NewAgent(LibP2P, a, MakeTestSeed(""))
		So(err, ShouldBeNil)
		_, n1, err := agent.NodeID()
		So(err, ShouldBeNil)
		err = agent.GenKeys(MakeTestSeed("seed1"))
		So(err, ShouldBeNil)
		_, n2, err := agent.NodeID()
		So(err, ShouldBeNil)
		So(n1, ShouldNotEqual, n2)
		err = agent.GenKeys(MakeTestSeed("seed1"))
		So(err, ShouldBeNil)
		_, n1, _ = agent.NodeID()
		So(n1, ShouldEqual, n2)
		err = agent.GenKeys(MakeTestSeed("different seed"))
		So(err, ShouldBeNil)
		_, n2, _ = agent.NodeID()
		So(n1, ShouldNotEqual, n2)
	})
}
