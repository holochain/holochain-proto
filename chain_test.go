package holochain

import (
	"bytes"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestNewChain(t *testing.T) {
	hashSpec, _, _ := chainTestSetup()
	Convey("it should make an empty chain", t, func() {
		c := NewChain(hashSpec)
		So(len(c.Headers), ShouldEqual, 0)
		So(len(c.Entries), ShouldEqual, 0)
	})

}

func TestNewChainFromFile(t *testing.T) {
	d := SetupTestDir()
	defer CleanupTestDir(d)
	hashSpec, key, now := chainTestSetup()

	var c *Chain
	var err error
	path := filepath.Join(d, "chain.dat")
	Convey("it should make an empty chain with encoder", t, func() {
		c, err = NewChainFromFile(hashSpec, path)
		So(err, ShouldBeNil)
		So(c.s, ShouldNotBeNil)
		So(FileExists(path), ShouldBeTrue)
	})

	e := GobEntry{C: "some data1"}
	c.AddEntry(now, "entryTypeFoo1", &e, key)
	e = GobEntry{C: "some other data2"}
	c.AddEntry(now, "entryTypeFoo2", &e, key)
	dump := c.String()
	c.s.Close()
	c, err = NewChainFromFile(hashSpec, path)
	Convey("it should load chain data if available", t, func() {
		So(err, ShouldBeNil)
		So(c.String(), ShouldEqual, dump)
	})

	e = GobEntry{C: "yet other data"}
	c.AddEntry(now, "yourData", &e, key)
	dump = c.String()
	c.s.Close()

	c, err = NewChainFromFile(hashSpec, path)
	Convey("should continue to append data after reload", t, func() {
		So(err, ShouldBeNil)
		So(c.String(), ShouldEqual, dump)
	})
}

func TestTop(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)
	var hash *Hash
	var hd *Header
	Convey("it should return an nil for an empty chain", t, func() {
		hd = c.Top()
		So(hd, ShouldBeNil)
		hash, hd = c.TopType("entryTypeFoo")
		So(hd, ShouldBeNil)
		So(hash, ShouldBeNil)
	})

	e := GobEntry{C: "some data"}
	c.AddEntry(now, "entryTypeFoo", &e, key)

	Convey("Top it should return the top header", t, func() {
		hd = c.Top()
		So(hd, ShouldEqual, c.Headers[0])
	})
	Convey("TopType should return nil for non existent type", t, func() {
		hash, hd = c.TopType("otherData")
		So(hd, ShouldBeNil)
		So(hash, ShouldBeNil)
	})
	Convey("TopType should return header for correct type", t, func() {
		hash, hd = c.TopType("entryTypeFoo")
		So(hd, ShouldEqual, c.Headers[0])
	})
	c.AddEntry(now, "otherData", &e, key)
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
	hashSpec, _, _ := chainTestSetup()
	c := NewChain(hashSpec)
	Convey("it should return nil for an empty chain", t, func() {
		hash, hd := c.TopType("entryTypeFoo")
		So(hd, ShouldBeNil)
		So(hash, ShouldBeNil)
	})
	Convey("it should return nil for an chain with no entries of the type", t, func() {
	})
}

func TestAddEntry(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)

	Convey("it should add nil to the chain", t, func() {
		e := GobEntry{C: "some data"}
		hash, err := c.AddEntry(now, "entryTypeFoo", &e, key)
		So(err, ShouldBeNil)
		So(len(c.Headers), ShouldEqual, 1)
		So(len(c.Entries), ShouldEqual, 1)
		So(c.TypeTops["entryTypeFoo"], ShouldEqual, 0)
		So(hash.Equal(&c.Hashes[0]), ShouldBeTrue)
	})
}

func TestGet(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)

	e1 := GobEntry{C: "some data"}
	h1, _ := c.AddEntry(now, "entryTypeFoo", &e1, key)
	hd1, err1 := c.Get(h1)

	e2 := GobEntry{C: "some other data"}
	h2, _ := c.AddEntry(now, "entryTypeFoo", &e2, key)
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

func TestMarshalChain(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)
	var emptyStringList []string

	e := GobEntry{C: "fake DNA"}
	c.AddEntry(now, DNAEntryType, &e, key)

	e = GobEntry{C: "fake agent entry"}
	c.AddEntry(now, AgentEntryType, &e, key)

	e = GobEntry{C: "some data"}
	c.AddEntry(now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(now, "entryTypeFoo2", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(now, "entryTypeFoo3", &e, key)

	e = GobEntry{C: "some private"}
	c.AddEntry(now, "entryTypePrivate", &e, key)

	Convey("it should be able to marshal and unmarshal full chain", t, func() {
		var b bytes.Buffer

		err := c.MarshalChain(&b, ChainMarshalFlagsNone, emptyStringList, emptyStringList)
		So(err, ShouldBeNil)
		flags, c1, err := UnmarshalChain(hashSpec, &b)
		So(err, ShouldBeNil)
		So(flags, ShouldEqual, ChainMarshalFlagsNone)
		So(c1.String(), ShouldEqual, c.String())

		// confirm that internal structures are properly set up
		for i := 0; i < len(c.Headers); i++ {
			So(c.Hashes[i].String(), ShouldEqual, c1.Hashes[i].String())
		}
		So(reflect.DeepEqual(c.TypeTops, c1.TypeTops), ShouldBeTrue)
		So(reflect.DeepEqual(c.Hmap, c1.Hmap), ShouldBeTrue)
		So(reflect.DeepEqual(c.Emap, c1.Emap), ShouldBeTrue)
		So(reflect.DeepEqual(c.Entries, c1.Entries), ShouldBeTrue)
	})

	Convey("it should be able to marshal and unmarshal specify types", t, func() {
		var b bytes.Buffer

		typeList := []string{AgentEntryType, "entryTypeFoo2"}
		err := c.MarshalChain(&b, ChainMarshalFlagsNone, typeList, emptyStringList)
		So(err, ShouldBeNil)
		flags, c1, err := UnmarshalChain(hashSpec, &b)
		So(err, ShouldBeNil)
		So(flags, ShouldEqual, ChainMarshalFlagsNone)
		So(len(c1.Entries), ShouldEqual, 3)
		So(c1.Headers[0].Type, ShouldEqual, DNAEntryType)
		So(c1.Headers[1].Type, ShouldEqual, AgentEntryType)
		So(c1.Headers[2].Type, ShouldEqual, "entryTypeFoo2")
		So(c1.TypeTops[AgentEntryType], ShouldEqual, 1)
		So(c1.TypeTops["entryTypeFoo2"], ShouldEqual, 2)
	})

	Convey("it should be able to marshal and unmarshal headers only", t, func() {
		var b bytes.Buffer

		err := c.MarshalChain(&b, ChainMarshalFlagsNoEntries, emptyStringList, emptyStringList)
		So(err, ShouldBeNil)
		flags, c1, err := UnmarshalChain(hashSpec, &b)
		So(err, ShouldBeNil)
		So(flags, ShouldEqual, ChainMarshalFlagsNoEntries)

		So(len(c1.Hashes), ShouldEqual, len(c.Hashes))
		So(len(c1.Entries), ShouldEqual, 0)

		// confirm that internal structures are properly set up
		for i := 0; i < len(c.Headers); i++ {
			So(c.Hashes[i].String(), ShouldEqual, c1.Hashes[i].String())
		}

		So(reflect.DeepEqual(c.TypeTops, c1.TypeTops), ShouldBeTrue)
		So(reflect.DeepEqual(c.Hmap, c1.Hmap), ShouldBeTrue)
		So(reflect.DeepEqual(c.Emap, c1.Emap), ShouldBeTrue)
	})

	Convey("it should be able to marshal and unmarshal entries only", t, func() {
		var b bytes.Buffer

		err := c.MarshalChain(&b, ChainMarshalFlagsNoHeaders, emptyStringList, emptyStringList)
		So(err, ShouldBeNil)
		flags, c1, err := UnmarshalChain(hashSpec, &b)
		So(err, ShouldBeNil)
		So(flags, ShouldEqual, ChainMarshalFlagsNoHeaders)

		So(len(c1.Hashes), ShouldEqual, 0)
		So(len(c1.Headers), ShouldEqual, 0)
		So(len(c1.Entries), ShouldEqual, len(c1.Entries))
		So(len(c1.Emap), ShouldEqual, 0)
		So(len(c1.TypeTops), ShouldEqual, 0)
		So(len(c1.Emap), ShouldEqual, 0)

		So(reflect.DeepEqual(c.Entries, c1.Entries), ShouldBeTrue)
	})

	Convey("it should be able to marshal and unmarshal with omitted DNA", t, func() {
		var b bytes.Buffer

		err := c.MarshalChain(&b, ChainMarshalFlagsOmitDNA, emptyStringList, emptyStringList)
		So(err, ShouldBeNil)
		flags, c1, err := UnmarshalChain(hashSpec, &b)
		So(err, ShouldBeNil)
		So(flags, ShouldEqual, ChainMarshalFlagsOmitDNA)
		So(c1.Entries[0].Content(), ShouldEqual, "")
		c1.Entries[0].(*GobEntry).C = c.Entries[0].(*GobEntry).C
		So(c1.String(), ShouldEqual, c.String())

		// confirm that internal structures are properly set up
		for i := 0; i < len(c.Headers); i++ {
			So(c.Hashes[i].String(), ShouldEqual, c1.Hashes[i].String())
		}
		So(reflect.DeepEqual(c.TypeTops, c1.TypeTops), ShouldBeTrue)
		So(reflect.DeepEqual(c.Hmap, c1.Hmap), ShouldBeTrue)
		So(reflect.DeepEqual(c.Emap, c1.Emap), ShouldBeTrue)
		So(reflect.DeepEqual(c.Entries, c1.Entries), ShouldBeTrue)

	})

	Convey("it should be able to marshal with contents of private entries being redacted", t, func() {
		var b bytes.Buffer

		privateTypes := []string{"entryTypePrivate"}
		err := c.MarshalChain(&b, ChainMarshalFlagsNone, emptyStringList, privateTypes)
		So(err, ShouldBeNil)
		_, c1, err := UnmarshalChain(hashSpec, &b)
		So(err, ShouldBeNil)
		So(len(c1.Headers), ShouldEqual, len(c.Headers))
		So(len(c1.Entries), ShouldEqual, len(c.Entries))
		So(c1.Entries[0].Content(), ShouldEqual, c.Entries[0].Content())
		So(c1.Entries[1].Content(), ShouldEqual, c.Entries[1].Content())
		So(c1.Entries[2].Content(), ShouldEqual, c.Entries[2].Content())
		So(c1.Entries[3].Content(), ShouldEqual, c.Entries[3].Content())
		So(c1.Entries[4].Content(), ShouldEqual, c.Entries[4].Content())
		So(c1.Entries[5].Content(), ShouldEqual, ChainMarshalPrivateEntryRedacted)

	})

}

func TestWalkChain(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)
	e := GobEntry{C: "some data"}
	c.AddEntry(now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(now, "entryTypeFoo2", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(now, "entryTypeFoo3", &e, key)

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
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)
	e := GobEntry{C: "some data"}
	c.AddEntry(now, DNAEntryType, &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(now, AgentEntryType, &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(now, "entryTypeFoo1", &e, key)

	Convey("it should validate", t, func() {
		So(c.Validate(false), ShouldBeNil)
	})

	Convey("it should fail to validate if we diddle some bits", t, func() {
		c.Entries[0].(*GobEntry).C = "fish" // tweak
		So(c.Validate(false).Error(), ShouldEqual, "entry hash mismatch at link 0")
		So(c.Validate(true), ShouldBeNil) // test skipping entry validation

		c.Entries[0].(*GobEntry).C = "some data" //restore
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		c.Headers[1].TypeLink = hash // tweak
		So(c.Validate(false).Error(), ShouldEqual, "header hash mismatch at link 1")

		c.Headers[1].TypeLink = NullHash() //restore
		c.Headers[0].Type = "entryTypeBar" //tweak
		err := c.Validate(false)
		So(err.Error(), ShouldEqual, "header hash mismatch at link 0")

		c.Headers[0].Type = DNAEntryType // restore
		t := c.Headers[0].Time           // tweak
		c.Headers[0].Time = time.Now()
		err = c.Validate(false)
		So(err.Error(), ShouldEqual, "header hash mismatch at link 0")

		c.Headers[0].Time = t                            // restore
		c.Headers[0].HeaderLink = c.Headers[0].EntryLink // tweak
		err = c.Validate(false)
		So(err.Error(), ShouldEqual, "header hash mismatch at link 0")

		c.Headers[0].HeaderLink = NullHash() // restore
		val := c.Headers[0].EntryLink.H[2]
		c.Headers[0].EntryLink.H[2] = 3 // tweak
		err = c.Validate(false)
		So(err.Error(), ShouldEqual, "header hash mismatch at link 0")

		c.Headers[0].EntryLink.H[2] = val // restore
		val = c.Headers[0].Sig.S[0]
		c.Headers[0].Sig.S[0] = 99 // tweak
		err = c.Validate(false)
		So(err.Error(), ShouldEqual, "header hash mismatch at link 0")

		c.Headers[0].Sig.S[0] = val        // restore
		c.Headers[0].Change.Action = "foo" // tweak
		err = c.Validate(false)
		So(err.Error(), ShouldEqual, "header hash mismatch at link 0")

	})
}

func TestChain2String(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)

	Convey("it should dump empty string for empty chain", t, func() {
		So(c.String(), ShouldEqual, "")
	})

	e := GobEntry{C: "some data"}
	c.AddEntry(now, DNAEntryType, &e, key)

	Convey("it should dump a chain to text", t, func() {
		So(c.String(), ShouldNotEqual, "")
	})
}

/*
func TestPersistingChain(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)
	var b bytes.Buffer
	c.encoder = gob.NewEncoder(&b)

	h, key, now := chainTestSetup()
	e := GobEntry{C: "some data"}
	c.AddEntry(now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "some other data"}
	c.AddEntry(now, "entryTypeFoo1", &e, key)

	e = GobEntry{C: "and more data"}
	c.AddEntry(now, "entryTypeFoo1", &e, key)

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
	a, _ := NewAgent(LibP2P, "agent id", MakeTestSeed(""))
	key = a.PrivKey()
	hc := Holochain{agent: a}
	dna := DNA{DHTConfig: DHTConfig{HashType: "sha2-256"}}
	hc.nucleus = NewNucleus(&hc, &dna)
	hP := &hc
	hP.PrepareHashType()
	hs = hP.hashSpec
	return
}
