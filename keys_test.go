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
