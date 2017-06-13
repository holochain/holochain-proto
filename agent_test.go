package holochain

import (
	ic "github.com/libp2p/go-libp2p-crypto"
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
		a1, err := NewAgent(IPFS, a)
		So(err, ShouldBeNil)
		err = SaveAgent(d, a1)
		So(err, ShouldBeNil)
		a2, err := LoadAgent(d)
		So(err, ShouldBeNil)
		So(a2.Name(), ShouldEqual, a1.Name())
		So(ic.KeyEqual(a1.PrivKey(), a2.PrivKey()), ShouldBeTrue)

	})
	Convey("it should fail to load an agent file that has bad permissions", t, func() {
		os.Chmod(d+"/"+PrivKeyFileName, OS_USER_RW)
		_, err := LoadAgent(d)
		So(err.Error(), ShouldEqual, d+"/"+PrivKeyFileName+" file not read-only")
	})
}
