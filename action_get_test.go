package holochain

import (
	"fmt"
	. "github.com/holochain/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestActionGet(t *testing.T) {
	nodesCount := 3
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()

	h := mt.nodes[0]

	e := GobEntry{C: "3"}
	hash, _ := e.Sum(h.hashSpec)

	Convey("receive should return not found if it doesn't exist", t, func() {
		m := h.node.NewMessage(GET_REQUEST, GetReq{H: hash})
		_, err := ActionReceiver(h, m)
		So(err, ShouldEqual, ErrHashNotFound)

		options := GetOptions{}
		a := ActionGet{GetReq{H: hash}, &options}
		fn := &APIFnGet{action: a}
		response, err := fn.Call(h)
		So(err, ShouldEqual, ErrHashNotFound)
		So(fmt.Sprintf("%v", response), ShouldEqual, "<nil>")

	})

	commit(h, "oddNumbers", "3")
	m := h.node.NewMessage(GET_REQUEST, GetReq{H: hash})
	Convey("receive should return value if it exists", t, func() {
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		So(resp.Entry.Content().(string), ShouldEqual, "3")
	})

	ringConnect(t, mt.ctx, mt.nodes, nodesCount)
	Convey("receive should return closer peers if it can", t, func() {
		h2 := mt.nodes[2]
		r, err := ActionReceiver(h2, m)
		So(err, ShouldBeNil)
		resp := r.(CloserPeersResp)
		So(len(resp.CloserPeers), ShouldEqual, 1)
		So(peer.ID(resp.CloserPeers[0].ID).Pretty(), ShouldEqual, "QmUfY4WeqD3UUfczjdkoFQGEgCAVNf7rgFfjdeTbr7JF1C")
	})

	Convey("get should return not found if hash doesn't exist and we are connected", t, func() {
		hash, err := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzfrom")
		if err != nil {
			panic(err)
		}

		options := GetOptions{}
		a := ActionGet{GetReq{H: hash}, &options}
		fn := &APIFnGet{action: a}
		response, err := fn.Call(h)
		So(err, ShouldEqual, ErrHashNotFound)
		So(response, ShouldBeNil)

	})

}
