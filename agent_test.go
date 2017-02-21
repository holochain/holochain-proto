package holochain

import (
	ic "github.com/libp2p/go-libp2p-crypto"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestAgent(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
	a := AgentID("zippy@someemail.com")

	Convey("it should fail to create an agent with an unknown key type", t, func() {
		_, err := NewAgent(99, a)
		So(err.Error(), ShouldEqual, "unknown key type: 99")
	})
	Convey("it should be create a new agent that is saved to a file and then loadable", t, func() {
		a1, err := NewAgent(IPFS, a)
		So(err, ShouldBeNil)
		err = SaveAgent(d, a1)
		So(err, ShouldBeNil)
		a2, err := LoadAgent(d)
		So(err, ShouldBeNil)
		So(a2.ID(), ShouldEqual, a1.ID())
		So(ic.KeyEqual(a1.PrivKey(), a2.PrivKey()), ShouldBeTrue)

	})
}
