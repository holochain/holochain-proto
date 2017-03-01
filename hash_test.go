package holochain

import (
	"fmt"
	mh "github.com/multiformats/go-multihash"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestHash(t *testing.T) {

	Convey("Hash string representation", t, func() {
		var h Hash
		var err error
		h.H, err = mh.Sum([]byte("test data"), mh.SHA2_256, -1)
		So(fmt.Sprintf("%v", h.H), ShouldEqual, "[18 32 145 111 0 39 165 117 7 76 231 42 51 23 119 195 71 141 101 19 247 134 165 145 189 137 45 161 165 119 191 35 53 249]")
		So(h.String(), ShouldEqual, "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		h, err = NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		So(err, ShouldBeNil)
		So(h.String(), ShouldEqual, "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		s := fmt.Sprintf("%v", h)
		So(s, ShouldEqual, "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	})
}

func TestNullHash(t *testing.T) {
	Convey("There should be a null hash", t, func() {
		h := NullHash()
		So(fmt.Sprintf("%v", h.H), ShouldEqual, "[0]")
		So(h.IsNullHash(), ShouldBeTrue)
	})

}

func TestEqual(t *testing.T) {
	h1, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	h2, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	nh1 := NullHash()
	nh2 := NullHash()
	Convey("similar hashes should equal", t, func() {
		So(h1.Equal(&h2), ShouldBeTrue)

		So(nh1.Equal(&nh2), ShouldBeTrue)
	})

	h3, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")

	Convey("dissimilar hashes should not", t, func() {
		So(h1.Equal(&h3), ShouldBeFalse)
		So(h1.Equal(&nh2), ShouldBeFalse)
	})

}
