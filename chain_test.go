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

func TestNewChainFromFile(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
	h, key, now := chainTestSetup()

	var c *Chain
	var err error
	path := d + "/chain.dat"
	Convey("it should make an empty chain with encoder", t, func() {
		c, err = NewChainFromFile(h, path)
		So(err, ShouldBeNil)
		So(c.s, ShouldNotBeNil)
		So(fileExists(path), ShouldBeTrue)
	})

	e := GobEntry{C: "some data1"}
	c.AddEntry(h, now, "entryTypeFoo1", &e, key)
	e = GobEntry{C: "some other data2"}
	c.AddEntry(h, now, "entryTypeFoo2", &e, key)
	dump := c.String()
	c.s.Close()
	c, err = NewChainFromFile(h, path)
	Convey("it should load chain data if available", t, func() {
		So(err, ShouldBeNil)
		So(c.String(), ShouldEqual, dump)
	})

	e = GobEntry{C: "yet other data"}
	c.AddEntry(h, now, "yourData", &e, key)
	dump = c.String()
	c.s.Close()

	c, err = NewChainFromFile(h, path)
	Convey("should continue to append data after reload", t, func() {
		So(err, ShouldBeNil)
		So(c.String(), ShouldEqual, dump)
	})
}

func TestTop(t *testing.T) {
	c := NewChain()
	var hash *Hash
	var hd *Header
	Convey("it should return an nil for an empty chain", t, func() {
		hd = c.Top()
		So(hd, ShouldBeNil)
		hash, hd = c.TopType("entryTypeFoo")
		So(hd, ShouldBeNil)
		So(hash, ShouldBeNil)
	})
	h, key, now := chainTestSetup()
	e := GobEntry{C: "some data"}
	c.AddEntry(h, now, "entryTypeFoo", &e, key)

	Convey("Top it should return the top header", t, func() {
		hd = c.Top()
		So(hd, ShouldEqual, c.Headers[0])
	})
	Convey("TopType should return nil for non existent type", t, func() {
		hash, hd = c.TopType("otherData")
		So(hd, ShouldBeNil)
		So(hash, ShouldEqual, nil)
	})
	Convey("TopType should return header for correct type", t, func() {
		hash, hd = c.TopType("entryTypeFoo")
		So(hd, ShouldEqual, c.Headers[0])
	})
	c.AddEntry(h, now, "otherData", &e, key)
	Convey("TopType should return headers for both types", t, func() {
		hash, hd = c.TopType("entryTypeFoo")
		So(hd, ShouldEqual, c.Headers[0])
		hash, hd = c.TopType("otherData")
		So(hd, ShouldEqual, c.Headers[1])
	})

	Convey("Nth should return the nth header", t, func() {
		hd = c.Nth(1)
		So(hd, ShouldEqual, c.Headers[0])
	})

}

func TestTopType(t *testing.T) {
	c := NewChain()
	Convey("it should return nil for an empty chain", t, func() {
		hash, hd := c.TopType("entryTypeFoo")
		So(hd, ShouldBeNil)
		So(hash, ShouldEqual, nil)
	})
	Convey("it should return nil for an chain with no entries of the type", t, func() {
	})
}

func TestAddEntry(t *testing.T) {
	c := NewChain()

	h, key, now := chainTestSetup()

	Convey("it should add nil to the chain", t, func() {
		e := GobEntry{C: "some data"}
		hash, err := c.AddEntry(h, now, "entryTypeFoo", &e, key)
		So(err, ShouldBeNil)
		So(len(c.Headers), ShouldEqual, 1)
		So(len(c.Entries), ShouldEqual, 1)
		So(c.TypeTops["entryTypeFoo"], ShouldEqual, 0)
		So(hash.Equal(&c.Hashes[0]), ShouldBeTrue)
	})
}

func TestGet(t *testing.T) {
	c := NewChain()
	h, key, now := chainTestSetup()

	e1 := GobEntry{C: "some data"}
	h1, _ := c.AddEntry(h, now, "entryTypeFoo", &e1, key)
	hd1, err1 := c.Get(h1)

	e2 := GobEntry{C: "some other data"}
	h2, _ := c.AddEntry(h, now, "entryTypeFoo", &e2, key)
	hd2, err2 := c.Get(h2)

	Convey("it should get header by hash or by Entry hash", t, func() {
		So(hd1, ShouldEqual, c.Headers[0])
		So(err1, ShouldBeNil)

		ehd, err := c.GetEntryHeader(hd1.EntryLink)
		So(ehd, ShouldEqual, c.Headers[0])
		So(err, ShouldBeNil)

		So(hd2, ShouldEqual, c.Headers[1])
		So(err2, ShouldBeNil)

		ehd, err = c.GetEntryHeader(hd2.EntryLink)
		So(ehd, ShouldEqual, c.Headers[1])
		So(err, ShouldBeNil)
	})

	Convey("it should get entry by hash", t, func() {
		ed, et, err := c.GetEntry(hd1.EntryLink)
		So(err, ShouldBeNil)
		So(et, ShouldEqual, "entryTypeFoo")
		So(fmt.Sprintf("%v", &e1), ShouldEqual, fmt.Sprintf("%v", ed))
		ed, et, err = c.GetEntry(hd2.EntryLink)
		So(err, ShouldBeNil)
		So(et, ShouldEqual, "entryTypeFoo")
		So(fmt.Sprintf("%v", &e2), ShouldEqual, fmt.Sprintf("%v", ed))
	})

	Convey("it should return nil for non existent hash", t, func() {
		hash, _ := NewHash("QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuUi")
		hd, err := c.Get(hash)
		So(hd, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})
}

func TestMarshal(t *testing.T) {
	c := NewChain()
	h, key, now := chainTestSetup()

	e := GobEntry{C: "some data"}
	c.AddEntry(h, now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(h, now, "entryTypeFoo2", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(h, now, "entryTypeFoo3", &e, key)

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
	c.AddEntry(h, now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(h, now, "entryTypeFoo2", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(h, now, "entryTypeFoo3", &e, key)

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
	c.AddEntry(h, now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(h, now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(h, now, "entryTypeFoo1", &e, key)

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

/*
func TestPersistingChain(t *testing.T) {
	c := NewChain()
	var b bytes.Buffer
	c.encoder = gob.NewEncoder(&b)

	h, key, now := chainTestSetup()
	e := GobEntry{C: "some data"}
	c.AddEntry(h, now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(h, now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(h, now, "entryTypeFoo1", &e, key)

	dec := gob.NewDecoder(&b)

	var header *Header
	var entry Entry
	header, entry, err := readPair(dec)

	Convey("it should have added items to the writer", t, func() {
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", header), ShouldEqual, fmt.Sprintf("%v", c.Headers[0]))
		So(fmt.Sprintf("%v", entry), ShouldEqual, fmt.Sprintf("%v", c.Entries[0]))
	})
}
*/

func chainTestSetup() (hs HashSpec, key ic.PrivKey, now time.Time) {
	a, _ := NewAgent(IPFS, "agent id")
	key = a.PrivKey()
	hc := Holochain{HashType: "sha2-256"}
	hP := &hc
	hP.PrepareHashType()
	hs = hP.hashSpec
	return
}
