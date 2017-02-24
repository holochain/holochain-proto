package holochain

import (
	"bytes"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	. "github.com/smartystreets/goconvey/convey"
	"reflect"
	"testing"
	"time"
)

func TestNewChain(t *testing.T) {
	Convey("it should make an empty chain", t, func() {
		c := NewChain()
		So(len(c.Headers), ShouldEqual, 0)
		So(len(c.Entries), ShouldEqual, 0)
	})
}

func TestNewHeader(t *testing.T) {
	h, key, now := chainTestSetup()

	Convey("it should make a header and return its hash", t, func() {
		e := GobEntry{C: "some data"}
		ph := NullHash()
		hash, header, err := newHeader(h, now, "myData", &e, key, ph, ph)
		So(err, ShouldBeNil)
		// encode the header and create a hash of it
		b, _ := ByteEncoder(header)
		var h2 Hash
		h2.Sum(h, b)
		So(h2.String(), ShouldEqual, hash.String())
	})
}

func TestTop(t *testing.T) {
	c := NewChain()
	Convey("it should return an nil for an empty chain", t, func() {
		hd := c.Top()
		So(hd, ShouldBeNil)
		hd = c.TopType("myData")
		So(hd, ShouldBeNil)
	})
	h, key, now := chainTestSetup()
	e := GobEntry{C: "some data"}
	c.AddEntry(h, now, "myData", &e, key)

	Convey("Top it should return the top header", t, func() {
		hd := c.Top()
		So(hd, ShouldEqual, c.Headers[0])
	})
	Convey("TopType should return nil for non existent type", t, func() {
		hd := c.TopType("otherData")
		So(hd, ShouldBeNil)
	})
	Convey("TopType should return header for correct type", t, func() {
		hd := c.TopType("myData")
		So(hd, ShouldEqual, c.Headers[0])
	})
	c.AddEntry(h, now, "otherData", &e, key)
	Convey("TopType should return headers for both types", t, func() {
		hd := c.TopType("myData")
		So(hd, ShouldEqual, c.Headers[0])
		hd = c.TopType("otherData")
		So(hd, ShouldEqual, c.Headers[1])
	})
}

func TestTopType(t *testing.T) {
	c := NewChain()
	Convey("it should return nil for an empty chain", t, func() {
		hd := c.TopType("myData")
		So(hd, ShouldBeNil)
	})
	Convey("it should return nil for an chain with no entries of the type", t, func() {
	})
}

func TestAddEntry(t *testing.T) {
	c := NewChain()

	h, key, now := chainTestSetup()

	Convey("it should add nil to the chain", t, func() {
		e := GobEntry{C: "some data"}
		hash, err := c.AddEntry(h, now, "myData", &e, key)
		So(err, ShouldBeNil)
		So(len(c.Headers), ShouldEqual, 1)
		So(len(c.Entries), ShouldEqual, 1)
		So(c.TypeTops["myData"], ShouldEqual, 0)
		So(hash.String(), ShouldEqual, c.Hashes[0].String())
	})
}

func TestGet(t *testing.T) {
	c := NewChain()
	h, key, now := chainTestSetup()

	e := GobEntry{C: "some data"}
	h1, _ := c.AddEntry(h, now, "myData", &e, key)

	e = GobEntry{C: "some other data"}
	h2, _ := c.AddEntry(h, now, "myData", &e, key)

	Convey("it should get data by hash", t, func() {
		hd := c.Get(h1)
		So(hd, ShouldEqual, c.Headers[0])
		hd = c.Get(h2)
		So(hd, ShouldEqual, c.Headers[1])
	})

	Convey("it should return nil for non existent hash", t, func() {
		hash, _ := NewHash("QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuUi")
		So(c.Get(hash), ShouldBeNil)
	})
}

func TestMarshal(t *testing.T) {
	c := NewChain()
	h, key, now := chainTestSetup()

	e := GobEntry{C: "some data"}
	c.AddEntry(h, now, "myData1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(h, now, "myData2", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(h, now, "myData3", &e, key)

	Convey("it should be able to marshal and unmarshal", t, func() {
		var b bytes.Buffer

		err := c.MarshalChain(&b)
		So(err, ShouldBeNil)
		c1, err := UnmarshalChain(&b)
		So(err, ShouldBeNil)
		So(c1.String(), ShouldEqual, c.String())

		// confirm that internal structures are properly set up
		for i := 0; i < len(c.Headers); i++ {
			So(c.Hashes[i].String(), ShouldEqual, c1.Hashes[i].String())
		}
		So(reflect.DeepEqual(c.TypeTops, c1.TypeTops), ShouldBeTrue)
		So(reflect.DeepEqual(c.Hmap, c1.Hmap), ShouldBeTrue)
		So(reflect.DeepEqual(c.Emap, c1.Emap), ShouldBeTrue)
	})
}

func TestWalkChain(t *testing.T) {
	c := NewChain()
	h, key, now := chainTestSetup()
	e := GobEntry{C: "some data"}
	c.AddEntry(h, now, "myData1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(h, now, "myData2", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(h, now, "myData3", &e, key)

	Convey("it should walk back from the top through all entries", t, func() {
		var x string
		var i int
		err := c.Walk(func(key *Hash, h *Header, entry Entry) error {
			i++
			x += fmt.Sprintf("%d:%v ", i, entry.(*GobEntry).C)
			return nil
		})
		So(err, ShouldBeNil)
		So(x, ShouldEqual, "1:and more data 2:some other data 3:some data ")
	})
}

func TestValidateChain(t *testing.T) {
	c := NewChain()
	h, key, now := chainTestSetup()
	e := GobEntry{C: "some data"}
	c.AddEntry(h, now, "myData1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(h, now, "myData1", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(h, now, "myData1", &e, key)

	Convey("it should validate", t, func() {
		So(c.Validate(h), ShouldBeNil)
	})

	Convey("it should fail to validate if we diddle some bits", t, func() {
		c.Entries[0].(*GobEntry).C = "fish"
		So(c.Validate(h).Error(), ShouldEqual, "entry hash mismatch at link 0")
		c.Entries[0].(*GobEntry).C = "some data"
		c.Headers[1].TypeLink = NullHash()
		So(c.Validate(h).Error(), ShouldEqual, "header hash mismatch at link 1")
	})
}

func chainTestSetup() (hP *Holochain, key ic.PrivKey, now time.Time) {
	a, _ := NewAgent(IPFS, "agent id")
	key = a.PrivKey()
	h := Holochain{HashType: "sha2-256"}
	hP = &h
	hP.PrepareHashType()
	return
}
