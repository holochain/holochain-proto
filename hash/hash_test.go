package hash

import (
	"bytes"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	mh "github.com/multiformats/go-multihash"
	. "github.com/smartystreets/goconvey/convey"
	"math/big"
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

func TestHashFromBytes(t *testing.T) {
	h1, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("it should make a hash from valid bytes", t, func() {
		h, err := HashFromBytes([]byte(h1.H))
		So(err, ShouldBeNil)
		So(h.Equal(&h1), ShouldBeTrue)
	})
}

func TestPeerIDFromHash(t *testing.T) {
	b58 := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2"
	h, _ := NewHash(b58)

	Convey("it should make a peerID from a Hash", t, func() {
		peer := PeerIDFromHash(h)
		So(peer.Pretty(), ShouldEqual, b58)
	})
}

func TestHashFromPeerID(t *testing.T) {
	b58 := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2"
	peer, _ := peer.IDB58Decode(b58)

	Convey("it should make a hash from a peer ID", t, func() {
		h := HashFromPeerID(peer)
		So(h.String(), ShouldEqual, b58)
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

func TestMarshalHash(t *testing.T) {
	Convey("should be able to marshal and unmarshal a hash", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		var b bytes.Buffer
		err := hash.MarshalHash(&b)
		So(err, ShouldBeNil)
		var hash2 Hash
		err = hash2.UnmarshalHash(&b)
		So(err, ShouldBeNil)
		So(hash.Equal(&hash), ShouldBeTrue)
	})
	Convey("should be able to marshal and unmarshal a Null Hash", t, func() {
		hash := NullHash()
		var b bytes.Buffer
		err := hash.MarshalHash(&b)
		So(err, ShouldBeNil)
		var hash2 Hash
		err = hash2.UnmarshalHash(&b)
		So(err, ShouldBeNil)
		So(hash.Equal(&hash), ShouldBeTrue)
	})
	Convey("should not be able to marshal and unmarshal a nil Hash", t, func() {
		var hash Hash
		var b bytes.Buffer
		err := hash.MarshalHash(&b)
		So(err.Error(), ShouldEqual, "can't marshal nil hash")
	})

}

func TestZeroPrefixLen(t *testing.T) {
	cases := [][]byte{
		{0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x58, 0xFF, 0x80, 0x00, 0x00, 0xF0},
	}
	lens := []int{24, 56, 9}

	Convey("it should calculate prefix lengths", t, func() {
		for i, c := range cases {
			r := ZeroPrefixLen(c)
			So(r, ShouldEqual, lens[i])
		}
	})
}

func TestHashXORDistance(t *testing.T) {
	h0, _ := HashFromBytes(
		[]byte{0x12, 0x20, 0x91, 0x6f, 0x00, 0x27, 0xa5, 0x75, 0x07, 0x4c, 0xe7, 0x2a, 0x33, 0x17, 0x77, 0xc3, 0x47, 0x8d, 0x65, 0x13, 0xf7, 0x86, 0xa5, 0x91, 0xbd, 0x89, 0x2d, 0xa1, 0xa5, 0x77, 0xbf, 0x23, 0x36, 0x00})
	h1, _ := HashFromBytes(
		[]byte{0x12, 0x20, 0x91, 0x6f, 0x00, 0x27, 0xa5, 0x75, 0x07, 0x4c, 0xe7, 0x2a, 0x33, 0x17, 0x77, 0xc3, 0x47, 0x8d, 0x65, 0x13, 0xf7, 0x86, 0xa5, 0x91, 0xbd, 0x89, 0x2d, 0xa1, 0xa5, 0x77, 0xbf, 0x23, 0x36, 0x01})
	h2, _ := HashFromBytes(
		[]byte{0x12, 0x20, 0x91, 0x6f, 0x00, 0x27, 0xa5, 0x75, 0x07, 0x4c, 0xe7, 0x2a, 0x33, 0x17, 0x77, 0xc3, 0x47, 0x8d, 0x65, 0x13, 0xf7, 0x86, 0xa5, 0x91, 0xbd, 0x89, 0x2d, 0xa1, 0xa5, 0x77, 0xbf, 0x23, 0x36, 0xff})
	Convey("the same hash should be at distance 0 from itself", t, func() {
		var d big.Int
		So(d.Cmp(HashXORDistance(h0, h0)), ShouldEqual, 0)
	})
	Convey("it should calculate distance", t, func() {
		So(fmt.Sprintf("%v", HashXORDistance(h0, h1)), ShouldEqual, "1")
		So(fmt.Sprintf("%v", HashXORDistance(h0, h2)), ShouldEqual, "255")
	})

}

func TestHashDistancesAndCenterSorting(t *testing.T) {

	hashes := makeTestHashes()

	cmp := func(a int64, b *big.Int) int {
		return big.NewInt(a).Cmp(b)
	}

	Convey("the same hash should be at distance 0 from itself", t, func() {
		So(cmp(0, HashXORDistance(hashes[2], hashes[3])), ShouldEqual, 0)
	})
	Convey("it should cmp if less", t, func() {
		So(cmp(1, HashXORDistance(hashes[2], hashes[4])), ShouldEqual, 0)
	})
	/*
		d1 := HashXORDistance(hashes[2], hashes[5])
		d2 := u.XOR(keys[2].Bytes, keys[5].Bytes)
		d2 = d2[len(keys[2].Bytes)-len(d1.Bytes()):] // skip empty space for big
		if !bytes.Equal(d1.Bytes(), d2) {
			t.Errorf("bytes should be the same. %v == %v", d1.Bytes(), d2)
		}*/
	/*
		if -1 != cmp(2<<32, keys[2].Distance(keys[5])) {
			t.Errorf("2<<32 should be smaller")
		}
	*/

	Convey("sort function should order by XOR distance", t, func() {
		hashes2 := SortByDistance(hashes[2], hashes)
		order := []int{2, 3, 4, 5, 1, 0}
		for i, o := range order {
			So(bytes.Equal(hashes[o].H, hashes2[i].H), ShouldBeTrue)
		}
	})
}

func makeTestHashes() []Hash {
	adjs := [][]byte{
		{0x12, 0x20, 173, 149, 19, 27, 192, 183, 153, 192, 177, 175, 71, 127, 177, 79, 207, 38, 166, 169, 247, 96, 121, 228, 139, 240, 144, 172, 183, 232, 54, 123, 253, 14},
		{0x12, 0x20, 223, 63, 97, 152, 4, 169, 47, 219, 64, 87, 25, 45, 196, 61, 215, 72, 234, 119, 138, 220, 82, 188, 73, 140, 232, 5, 36, 192, 20, 184, 17, 25},
		{0x12, 0x20, 73, 176, 221, 176, 149, 143, 22, 42, 129, 124, 213, 114, 232, 95, 189, 154, 18, 3, 122, 132, 32, 199, 53, 185, 58, 157, 117, 78, 52, 146, 157, 127},
		{0x12, 0x20, 73, 176, 221, 176, 149, 143, 22, 42, 129, 124, 213, 114, 232, 95, 189, 154, 18, 3, 122, 132, 32, 199, 53, 185, 58, 157, 117, 78, 52, 146, 157, 127},
		{0x12, 0x20, 73, 176, 221, 176, 149, 143, 22, 42, 129, 124, 213, 114, 232, 95, 189, 154, 18, 3, 122, 132, 32, 199, 53, 185, 58, 157, 117, 78, 52, 146, 157, 126},
		{0x12, 0x20, 73, 0, 221, 176, 149, 143, 22, 42, 129, 124, 213, 114, 232, 95, 189, 154, 18, 3, 122, 132, 32, 199, 53, 185, 58, 157, 117, 78, 52, 146, 157, 127},
	}

	hashes := make([]Hash, len(adjs))
	var err error
	for i, a := range adjs {
		hashes[i], err = HashFromBytes(a)
		if err != nil {
			panic(err)
		}
	}
	return hashes
}
