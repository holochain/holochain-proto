package holochain

import (
  "testing"
  . "github.com/smartystreets/goconvey/convey"
)

func TestAgentEntryToJSON(t *testing.T) {
	a := AgentIdentity("zippy@someemail.com")
	a1, _ := NewAgent(LibP2P, a, MakeTestSeed(""))
	pk, _ := a1.EncodePubKey()
	ae := AgentEntry{
		Identity:  a,
		PublicKey: pk,
	}

	var j string
	var err error
	Convey("it should convert to JSON", t, func() {
		j, err = ae.ToJSON()
		So(err, ShouldBeNil)
		So(j, ShouldEqual, `{"Identity":"zippy@someemail.com","Revocation":"","PublicKey":"4XTTM4Lf8pAWo6dfra223t4ZK7gjAjFA49VdwrC1wVHQqb8nH"}`)
	})

	Convey("it should convert from JSON", t, func() {
		ae2, err := AgentEntryFromJSON(j)
		So(err, ShouldBeNil)
		So(ae2, ShouldResemble, ae)
	})

}
