package holochain

import (
  "fmt"
  . "github.com/smartystreets/goconvey/convey"
  "testing"
)

func TestActionCommit(t *testing.T) {
	nodesCount := 3
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()

	h := mt.nodes[0]

	profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)
	linksHash := commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, h.nodeIDStr, profileHash.String()))
	ringConnect(t, mt.ctx, mt.nodes, nodesCount)

	Convey("when committing a link the linkEntry itself should be published to the DHT", t, func() {
		req := GetReq{H: linksHash, GetMask: GetMaskEntry}
		_, err := callGet(h, req, &GetOptions{GetMask: req.GetMask})
		So(err, ShouldBeNil)

		h2 := mt.nodes[2]
		_, err = callGet(h2, req, &GetOptions{GetMask: req.GetMask})
		So(err, ShouldBeNil)

	})
}
