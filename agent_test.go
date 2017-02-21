package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestKeys(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)

	Convey("generating keys should marshal key files that can be unmarshaled", t, func() {
		p, err := GenKeys(d)
		So(err, ShouldEqual, nil)
		p2, err := UnmarshalPrivateKey(d, PrivKeyFileName)
		So(err, ShouldEqual, nil)
		So(fmt.Sprintf("%v", p), ShouldEqual, fmt.Sprintf("%v", p2))
		pub, err := UnmarshalPublicKey(d, PubKeyFileName)
		So(fmt.Sprintf("%v", p.Public()), ShouldEqual, fmt.Sprintf("%v", pub))
	})
}

func TestAgent(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)

	Convey("it should be create a new agent that is saved to a file and then loadable", t, func() {
		a1 := AgentID("zippy@someemail.com")
		p1, err := NewAgent(d, a1)
		So(err, ShouldBeNil)
		a2, p2, err := LoadAgent(d)
		So(err, ShouldBeNil)
		So(a2, ShouldEqual, a1)
		So(fmt.Sprintf("%v", p2), ShouldEqual, fmt.Sprintf("%v", p1))
	})
}
