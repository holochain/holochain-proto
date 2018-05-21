package holochain

import (
  . "github.com/smartystreets/goconvey/convey"
  "testing"
  . "github.com/holochain/holochain-proto/hash"
)

func TestActionDelete(t *testing.T) {
	nodesCount := 3
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()

	h := mt.nodes[0]
	ringConnect(t, mt.ctx, mt.nodes, nodesCount)

	profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)
	entry := DelEntry{Hash: profileHash, Message: "expired"}
	a := &ActionDel{entry: entry}
	response, err := h.commitAndShare(a, NullHash())
	if err != nil {
		panic(err)
	}
	deleteHash := response.(Hash)

	Convey("when deleting a hash the del entry itself should be published to the DHT", t, func() {
		req := GetReq{H: deleteHash, GetMask: GetMaskEntry}
		_, err := callGet(h, req, &GetOptions{GetMask: req.GetMask})
		So(err, ShouldBeNil)

		h2 := mt.nodes[2]
		_, err = callGet(h2, req, &GetOptions{GetMask: req.GetMask})
		So(err, ShouldBeNil)
	})
}
