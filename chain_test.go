package holochain

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/HC-Interns/holochain-proto/hash"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChainNew(t *testing.T) {
	hashSpec, _, _ := chainTestSetup()
	Convey("it should make an empty chain", t, func() {
		c := NewChain(hashSpec)
		So(len(c.Headers), ShouldEqual, 0)
		So(len(c.Entries), ShouldEqual, 0)
	})

}

func TestChainNewChainFromFile(t *testing.T) {
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

func TestChainTop(t *testing.T) {
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

func TestChainTopType(t *testing.T) {
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

func TestChainAddEntry(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)

	Convey("it should add nil to the chain", t, func() {
		e := GobEntry{C: "some data"}
		hash, err := c.AddEntry(now, "entryTypeFoo", &e, key)
		So(err, ShouldBeNil)
		So(len(c.Headers), ShouldEqual, 1)
		So(len(c.Entries), ShouldEqual, 1)
		So(c.TypeTops["entryTypeFoo"], ShouldEqual, 0)
		So(hash.Equal(c.Hashes[0]), ShouldBeTrue)
	})
}

func TestChainGet(t *testing.T) {
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

func TestChainMarshalChain(t *testing.T) {
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

func TestChainValidateChain(t *testing.T) {
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
		before := c.Headers[0].EntryLink
		tweak := []byte(before)
		tweak[5] = 3 // tweak
		c.Headers[0].EntryLink = Hash(tweak)
		err = c.Validate(false)
		So(err.Error(), ShouldEqual, "header hash mismatch at link 0")

		c.Headers[0].EntryLink = before // restore
		val := c.Headers[0].Sig.S[0]
		c.Headers[0].Sig.S[0] = 99 // tweak
		err = c.Validate(false)
		So(err.Error(), ShouldEqual, "header hash mismatch at link 0")

		c.Headers[0].Sig.S[0] = val // restore
		c.Headers[0].Change = "foo" // tweak
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

func TestChain2JSON(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)

	Convey("it should dump empty JSON object for empty chain", t, func() {
		json, err := c.JSON(0)
		So(err, ShouldBeNil)
		So(json, ShouldEqual, "{}")
	})

	e := GobEntry{C: "dna entry"}
	c.AddEntry(now, DNAEntryType, &e, key)

	Convey("it should dump a DNA entry as JSON", t, func() {
		json, err := c.JSON(0)
		So(err, ShouldBeNil)
		json = NormaliseJSON(json)
		matched, err := regexp.MatchString(`{"%dna":{"header":{.*},"content":"dna entry"}}`, json)
		So(err, ShouldBeNil)
		So(matched, ShouldBeTrue)
	})

	e = GobEntry{C: "agent entry"}
	c.AddEntry(now, AgentEntryType, &e, key)

	Convey("it should dump a Agent entry as JSON", t, func() {
		json, err := c.JSON(0)
		So(err, ShouldBeNil)
		json = NormaliseJSON(json)
		matched, err := regexp.MatchString(`{"%dna":{"header":{.*},"content":"dna entry"},"%agent":{"header":{.*},"content":"agent entry"}}`, json)
		So(err, ShouldBeNil)
		So(matched, ShouldBeTrue)
	})

	e = GobEntry{C: "chain entry"}
	c.AddEntry(now, "handle", &e, key)

	Convey("it should dump chain with entries as JSON", t, func() {
		json, err := c.JSON(0)
		So(err, ShouldBeNil)
		json = NormaliseJSON(json)
		matched, err := regexp.MatchString(`{"%dna":{.*},"%agent":{.*},"entries":\[{"header":{"type":"handle",.*"},"content":"chain entry"}\]}`, json)
		So(err, ShouldBeNil)
		So(matched, ShouldBeTrue)
	})

	e.C = "chain entry 2"
	c.AddEntry(now, "handle", &e, key)
	e.C = "chain entry 3"
	c.AddEntry(now, "handle", &e, key)
	Convey("it should dump chain from the given start index", t, func() {
		json, err := c.JSON(2)
		So(err, ShouldBeNil)
		json = NormaliseJSON(json)
		matched, err := regexp.MatchString(`{"entries":\[{"header":{"type":"handle",.*"},"content":"chain entry 2"},{"header":{"type":"handle",.*"},"content":"chain entry 3"}\]}`, json)
		So(err, ShouldBeNil)
		So(matched, ShouldBeTrue)
	})
}

func TestChain2Dot(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)

	Convey("it should dump an empty 'dot' document for empty chain", t, func() {
		dot, err := c.Dot(0)
		So(err, ShouldBeNil)
		matched, err := regexp.MatchString(`digraph chain {.*}`, strings.Replace(dot, "\n", "", -1))
		So(err, ShouldBeNil)
		So(matched, ShouldBeTrue)
		So(dot, ShouldNotContainSubstring, "header")
		So(dot, ShouldNotContainSubstring, "content")
	})

	e := GobEntry{C: "dna entry"}
	c.AddEntry(now, DNAEntryType, &e, key)

	Convey("after adding the dna, the dump should include the genesis entry in 'dot' format", t, func() {
		dot, err := c.Dot(0)
		So(err, ShouldBeNil)

		hdr := c.Headers[0]
		timestamp := fmt.Sprintf("%v", hdr.Time)
		hdrType := fmt.Sprintf("%v", hdr.Type)
		hdrEntry := fmt.Sprintf("%v", hdr.EntryLink)
		nextHeader := fmt.Sprintf("%v", hdr.HeaderLink)
		next := fmt.Sprintf("%s: %v", hdr.Type, hdr.TypeLink)
		hash := fmt.Sprintf("%s", c.Hashes[0])

		expectedDot := `header0 [label=<{HEADER 0: GENESIS|
{Type|` + hdrType + `}|
{Hash|` + hash + `}|
{Timestamp|` + timestamp + `}|
{Next Header|` + nextHeader + `}|
{Next|` + next + `}|
{Entry|` + hdrEntry + `}
}>];
content0 [label=<{HOLOCHAIN DNA|See dna.json}>];
header0->content0;`

		So(dot, ShouldContainSubstring, expectedDot)
	})

	e = GobEntry{C: `{"Identity":"lucy","Revocation":"","PublicKey":"XYZ"}`}
	c.AddEntry(now, AgentEntryType, &e, key)

	Convey("after adding the agent, the dump should include the agent entry in 'dot' format", t, func() {
		dot, err := c.Dot(0)
		So(err, ShouldBeNil)

		hdr0 := c.Headers[0]
		timestamp0 := fmt.Sprintf("%v", hdr0.Time)
		hdrType0 := fmt.Sprintf("%v", hdr0.Type)
		hdrEntry0 := fmt.Sprintf("%v", hdr0.EntryLink)
		nextHeader0 := fmt.Sprintf("%v", hdr0.HeaderLink)
		next0 := fmt.Sprintf("%s: %v", hdr0.Type, hdr0.TypeLink)
		hash0 := fmt.Sprintf("%s", c.Hashes[0])

		hdr1 := c.Headers[1]
		timestamp1 := fmt.Sprintf("%v", hdr1.Time)
		hdrType1 := fmt.Sprintf("%v", hdr1.Type)
		hdrEntry1 := fmt.Sprintf("%v", hdr1.EntryLink)
		nextHeader1 := fmt.Sprintf("%v", hdr1.HeaderLink)
		next1 := fmt.Sprintf("%s: %v", hdr1.Type, hdr1.TypeLink)
		hash1 := fmt.Sprintf("%s", c.Hashes[1])

		expectedDot := `header0 [label=<{HEADER 0: GENESIS|
{Type|` + hdrType0 + `}|
{Hash|` + hash0 + `}|
{Timestamp|` + timestamp0 + `}|
{Next Header|` + nextHeader0 + `}|
{Next|` + next0 + `}|
{Entry|` + hdrEntry0 + `}
}>];
content0 [label=<{HOLOCHAIN DNA|See dna.json}>];
header0->content0;
header0->header1;
header1 [label=<{HEADER 1|
{Type|` + hdrType1 + `}|
{Hash|` + hash1 + `}|
{Timestamp|` + timestamp1 + `}|
{Next Header|` + nextHeader1 + `}|
{Next|` + next1 + `}|
{Entry|` + hdrEntry1 + `}
}>];
content1 [label=<{AGENT ID|\{"Identity":"lucy",<br/>"Revocation":"",<br/>"PublicKey":"XYZ"\}}>];
header1->content1;`

		So(dot, ShouldContainSubstring, expectedDot)
	})

	e = GobEntry{C: `{"Links":[{"Base":"ABC","Link":"XYZ","Tag":"handle"}]}`}
	c.AddEntry(now, "handle", &e, key)

	Convey("after adding an entry, the dump should include the entry in 'dot' format", t, func() {
		dot, err := c.Dot(0)
		So(err, ShouldBeNil)

		hdr0 := c.Headers[0]
		timestamp0 := fmt.Sprintf("%v", hdr0.Time)
		hdrType0 := fmt.Sprintf("%v", hdr0.Type)
		hdrEntry0 := fmt.Sprintf("%v", hdr0.EntryLink)
		nextHeader0 := fmt.Sprintf("%v", hdr0.HeaderLink)
		next0 := fmt.Sprintf("%s: %v", hdr0.Type, hdr0.TypeLink)
		hash0 := fmt.Sprintf("%s", c.Hashes[0])

		hdr1 := c.Headers[1]
		timestamp1 := fmt.Sprintf("%v", hdr1.Time)
		hdrType1 := fmt.Sprintf("%v", hdr1.Type)
		hdrEntry1 := fmt.Sprintf("%v", hdr1.EntryLink)
		nextHeader1 := fmt.Sprintf("%v", hdr1.HeaderLink)
		next1 := fmt.Sprintf("%s: %v", hdr1.Type, hdr1.TypeLink)
		hash1 := fmt.Sprintf("%s", c.Hashes[1])

		hdr2 := c.Headers[2]
		timestamp2 := fmt.Sprintf("%v", hdr2.Time)
		hdrType2 := fmt.Sprintf("%v", hdr2.Type)
		hdrEntry2 := fmt.Sprintf("%v", hdr2.EntryLink)
		nextHeader2 := fmt.Sprintf("%v", hdr2.HeaderLink)
		next2 := fmt.Sprintf("%s: %v", hdr2.Type, hdr2.TypeLink)
		hash2 := fmt.Sprintf("%s", c.Hashes[2])

		expectedDot := `header0 [label=<{HEADER 0: GENESIS|
{Type|` + hdrType0 + `}|
{Hash|` + hash0 + `}|
{Timestamp|` + timestamp0 + `}|
{Next Header|` + nextHeader0 + `}|
{Next|` + next0 + `}|
{Entry|` + hdrEntry0 + `}
}>];
content0 [label=<{HOLOCHAIN DNA|See dna.json}>];
header0->content0;
header0->header1;
header1 [label=<{HEADER 1|
{Type|` + hdrType1 + `}|
{Hash|` + hash1 + `}|
{Timestamp|` + timestamp1 + `}|
{Next Header|` + nextHeader1 + `}|
{Next|` + next1 + `}|
{Entry|` + hdrEntry1 + `}
}>];
content1 [label=<{AGENT ID|\{"Identity":"lucy",<br/>"Revocation":"",<br/>"PublicKey":"XYZ"\}}>];
header1->content1;
header1->header2;
header2 [label=<{HEADER 2|
{Type|` + hdrType2 + `}|
{Hash|` + hash2 + `}|
{Timestamp|` + timestamp2 + `}|
{Next Header|` + nextHeader2 + `}|
{Next|` + next2 + `}|
{Entry|` + hdrEntry2 + `}
}>];
content2 [label=<{ENTRY 2|\{"Links":[<br/>\{"Base":"ABC",<br/>"Link":"XYZ",<br/>"Tag":"handle"\}]\}}>];
header2->content2;`

		So(dot, ShouldContainSubstring, expectedDot)
	})

	Convey("only entries starting from the specified index should be dumped", t, func() {
		dot, err := c.Dot(2)
		So(err, ShouldBeNil)

		hdr2 := c.Headers[2]
		timestamp2 := fmt.Sprintf("%v", hdr2.Time)
		hdrType2 := fmt.Sprintf("%v", hdr2.Type)
		hdrEntry2 := fmt.Sprintf("%v", hdr2.EntryLink)
		nextHeader2 := fmt.Sprintf("%v", hdr2.HeaderLink)
		next2 := fmt.Sprintf("%s: %v", hdr2.Type, hdr2.TypeLink)
		hash2 := fmt.Sprintf("%s", c.Hashes[2])

		expectedDot := `header2 [label=<{HEADER 2|
{Type|` + hdrType2 + `}|
{Hash|` + hash2 + `}|
{Timestamp|` + timestamp2 + `}|
{Next Header|` + nextHeader2 + `}|
{Next|` + next2 + `}|
{Entry|` + hdrEntry2 + `}
}>];
content2 [label=<{ENTRY 2|\{"Links":[<br/>\{"Base":"ABC",<br/>"Link":"XYZ",<br/>"Tag":"handle"\}]\}}>];
header2->content2;`

		So(dot, ShouldContainSubstring, expectedDot)
	})
}

func TestChainBundle(t *testing.T) {
	hashSpec, key, now := chainTestSetup()
	c := NewChain(hashSpec)
	e := GobEntry{C: "fake DNA"}
	c.AddEntry(now, DNAEntryType, &e, key)
	e = GobEntry{C: "foo data"}
	c.AddEntry(now, "entryTypeFoo2", &e, key)

	Convey("starting a bundle should set the bundle start point", t, func() {
		So(c.BundleStarted(), ShouldBeNil)
		err := c.StartBundle("myBundle")
		So(err, ShouldBeNil)
		bundle := c.BundleStarted()
		So(bundle, ShouldNotBeNil)
		So(bundle.idx, ShouldEqual, c.Length()-1)
		So(bundle.userParam, ShouldEqual, `"myBundle"`) // should convert user param to json
		So(bundle.chain.bundleOf, ShouldEqual, c)
	})

	Convey("it should add entries to the bundle chain", t, func() {

		e := GobEntry{C: "some data"}

		bundle := c.BundleStarted()
		So(bundle.chain.Length(), ShouldEqual, 0)

		now := now.Round(0)
		l, hash, header, err := bundle.chain.prepareHeader(now, "entryTypeFoo1", &e, key, NullHash())
		So(err, ShouldBeNil)
		So(l, ShouldEqual, 0)

		err = bundle.chain.addEntry(l, hash, header, &e)
		So(err, ShouldBeNil)
		So(bundle.chain.Length(), ShouldEqual, 1)

		e = GobEntry{C: "another entry"}
		_, err = bundle.chain.AddEntry(now, "entryTypeFoo2", &e, key)
		So(err, ShouldBeNil)
		So(bundle.chain.Length(), ShouldEqual, 2)
	})

	Convey("you shouldn't be able to work on a chain when bundle opened", t, func() {
		l, hash, header, err := c.prepareHeader(now, "entryTypeFoo1", &e, key, NullHash())
		So(err, ShouldEqual, ErrChainLockedForBundle)

		err = c.addEntry(l, hash, header, &e)
		So(err, ShouldEqual, ErrChainLockedForBundle)
	})

	Convey("it should add entries to the main chain when bundle closed and validate!", t, func() {
		So(c.Length(), ShouldEqual, 2)
		err := c.CloseBundle(true)
		So(err, ShouldBeNil)
		So(c.BundleStarted(), ShouldBeNil)
		So(c.Length(), ShouldEqual, 4)
		So(c.Validate(false), ShouldBeNil)

		// makes sure type linking worked too
		hash, _ := c.TopType("entryTypeFoo1")
		So(hash.String(), ShouldEqual, c.Hashes[2].String())
		hash, _ = c.TopType("entryTypeFoo2")
		So(hash.String(), ShouldEqual, c.Hashes[3].String())

	})

	Convey("it should not add entries to the main chain when bundle closed without commit!", t, func() {
		So(c.Length(), ShouldEqual, 4)
		err := c.StartBundle("myBundle")
		e = GobEntry{C: "another entry"}
		_, err = c.bundle.chain.AddEntry(now, "entryTypeFoo2", &e, key)
		So(c.bundle.chain.Length(), ShouldEqual, 1)
		err = c.CloseBundle(false)
		So(err, ShouldBeNil)
		So(c.Length(), ShouldEqual, 4)
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
