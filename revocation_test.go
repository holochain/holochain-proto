package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestSelfRevocationVerify(t *testing.T) {
	_, oldPrivKey := makePeer("peer1")
	_, newPrivKey := makePeer("peer2")

	revocation, err := NewSelfRevocation(oldPrivKey, newPrivKey, []byte("extra data"))

	Convey("verify should check the revocation", t, func() {
		So(err, ShouldBeNil)
		err = revocation.Verify()
		So(err, ShouldBeNil)
	})

	Convey("verify should fail on modified data", t, func() {
		//x := revocation.data[len(revocation.data)-3]
		revocation.Data[len(revocation.Data)-3] = 1
		err := revocation.Verify()
		So(err, ShouldEqual, SelfRevocationDoesNotVerify)
	})
}

func TestSelfRevocationMarshal(t *testing.T) {
	_, oldPrivKey := makePeer("peer1")
	_, newPrivKey := makePeer("peer2")

	revocation, _ := NewSelfRevocation(oldPrivKey, newPrivKey, []byte("extra data"))

	Convey("should marshal and unmarshal", t, func() {
		data, err := revocation.Marshal()
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, `{"Data":"JAgBEiC8/nwPO4mS3MSCLuHPqfQO2ffxwHUvVV0f9e1GjtihlggBEiBFvAl5ouxGA7GzS1vDHeB7CmdHkTq9RE6ojWuZ/b03KGV4dHJhIGRhdGE=","OldSig":"x7YRx7qrxd5Csh0xi1O4yi4BEO+Bn26gNoi0rLf1+QKH2BzYcVtczZXDTJi/C7r+RHOTUrY09AYHVIy/bOCCBA==","NewSig":"xVBsmxp+5y/Kr8TqQ5EfjJKJa4Q162eQPI3bNYJjqwk5HdHcXuO8xlk7cuCWUGnHBr1IGI5D6L0IEdiwBRPEDQ=="}`)

		newr := &SelfRevocation{}

		err = newr.Unmarshal(data)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", newr), ShouldEqual, fmt.Sprintf("%v", revocation))
	})
}
