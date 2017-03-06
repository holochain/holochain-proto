package holochain

import (
	"bytes"
	gob "encoding/gob"
	"fmt"
	toml "github.com/BurntSushi/toml"
	"github.com/google/uuid"
	ic "github.com/libp2p/go-libp2p-crypto"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	a, _ := NewAgent(IPFS, "Joe")

	Convey("New should fill Holochain struct with provided values and new UUID", t, func() {

		h := New(a, "some/path", "json")
		nID := string(uuid.NodeID())
		So(nID, ShouldEqual, string(h.Id.NodeID()))
		So(h.agent.ID(), ShouldEqual, "Joe")
		So(h.agent.PrivKey(), ShouldEqual, a.PrivKey())
		So(h.path, ShouldEqual, "some/path")
		So(h.encodingFormat, ShouldEqual, "json")
	})
	Convey("New with Zome should fill them", t, func() {
		z := Zome{Name: "myZome",
			Description: "zome desc",
			Code:        "zome_myZome.zy",
			Entries: map[string]EntryDef{
				"myData1": EntryDef{Name: "myData1", DataFormat: "string"},
				"myData2": EntryDef{Name: "myData2", DataFormat: "zygo"},
			},
		}

		h := New(a, "some/path", "yaml", z)
		nz := h.Zomes["myZome"]
		So(nz.Description, ShouldEqual, "zome desc")
		So(nz.Code, ShouldEqual, "zome_myZome.zy")
		So(fmt.Sprintf("%v", nz.Entries["myData1"]), ShouldEqual, "{myData1 string   <nil>}")
		So(fmt.Sprintf("%v", nz.Entries["myData2"]), ShouldEqual, "{myData2 zygo   <nil>}")
	})

}

func TestPrepareHashType(t *testing.T) {

	Convey("A bad hash type should return an error", t, func() {
		h := Holochain{HashType: "bogus"}
		err := h.PrepareHashType()
		So(err.Error(), ShouldEqual, "Unknown hash type: bogus")
	})
	Convey("It should initialized fixed and variable sized hashes", t, func() {
		h := Holochain{HashType: "sha1"}
		err := h.PrepareHashType()
		So(err, ShouldBeNil)
		var hash Hash
		err = hash.Sum(h.hashSpec, []byte("test data"))
		So(err, ShouldBeNil)
		So(hash.String(), ShouldEqual, "5duC28CW416wX42vses7TeTeRYwku9")

		h.HashType = "blake2b-256"
		err = h.PrepareHashType()
		So(err, ShouldBeNil)
		err = hash.Sum(h.hashSpec, []byte("test data"))
		So(err, ShouldBeNil)
		So(hash.String(), ShouldEqual, "2DrjgbL49zKmX4P7UgdopSCC7MhfVUySNbRHBQzdDuXgaJSNEg")
	})
}

func TestGenDev(t *testing.T) {
	d, s := setupTestService()
	defer cleanupTestDir(d)
	name := "test"
	root := s.Path + "/" + name

	Convey("we detected unconfigured holochains", t, func() {
		f, err := s.IsConfigured(name)
		So(f, ShouldEqual, "")
		So(err.Error(), ShouldEqual, "DNA not found")
		_, err = s.load("test", "json")
		So(err.Error(), ShouldEqual, "open "+root+"/"+DNAFileName+".json: no such file or directory")

	})

	Convey("when generating a dev holochain", t, func() {
		h, err := s.GenDev(root, "json")
		So(err, ShouldBeNil)
		h.store.Close()

		f, err := s.IsConfigured(name)
		So(err, ShouldBeNil)
		So(f, ShouldEqual, "json")

		h, err = s.Load(name)
		So(err, ShouldBeNil)
		h.store.Close()

		lh, err := s.load(name, "json")
		So(err, ShouldBeNil)
		So(lh.ID, ShouldEqual, h.ID)
		So(lh.config.Port, ShouldEqual, DefaultPort)
		So(h.config.PeerModeDHTNode, ShouldEqual, s.Settings.DefaultPeerModeDHTNode)
		So(h.config.PeerModeAuthor, ShouldEqual, s.Settings.DefaultPeerModeAuthor)
		lh.store.Close()

		So(fileExists(h.path+"/schema_profile.json"), ShouldBeTrue)
		So(fileExists(h.path+"/ui/index.html"), ShouldBeTrue)
		So(fileExists(h.path+"/"+ConfigFileName+".json"), ShouldBeTrue)

		Convey("we should not be able re generate it", func() {
			_, err = s.GenDev(root, "json")
			So(err.Error(), ShouldEqual, "holochain: "+root+" already exists")
		})
	})
}

func TestClone(t *testing.T) {
	d, s, _ := setupTestChain("test")
	defer cleanupTestDir(d)

	name := "test2"
	root := s.Path + "/" + name

	orig := s.Path + "/test"
	Convey("it should create a chain from the examples directory", t, func() {
		h, err := s.Clone(orig, root)
		So(err, ShouldBeNil)
		So(h.Name, ShouldEqual, "test2")
		agent, err := LoadAgent(s.Path)
		So(err, ShouldBeNil)
		So(h.agent.ID(), ShouldEqual, agent.ID())
		So(ic.KeyEqual(h.agent.PrivKey(), agent.PrivKey()), ShouldBeTrue)
		src, _ := readFile(orig, "zome_myZome.zy")
		dst, _ := readFile(root, "zome_myZome.zy")
		So(string(src), ShouldEqual, string(dst))
		So(fileExists(h.path+"/ui/index.html"), ShouldBeTrue)
		So(fileExists(h.path+"/schema_profile.json"), ShouldBeTrue)
		So(fileExists(h.path+"/schema_properties.json"), ShouldBeTrue)
		So(fileExists(h.path+"/"+ConfigFileName+".toml"), ShouldBeTrue)
	})
}

func TestNewEntry(t *testing.T) {
	d, s := setupTestService()
	defer cleanupTestDir(d)
	n := "test"
	path := s.Path + "/" + n
	h, err := s.GenDev(path, "toml")
	if err != nil {
		panic(err)
	}

	myData := `(message (from "art") (to "eric") (contents "test"))`

	now := time.Unix(1, 1) // pick a constant time so the test will always work

	e := GobEntry{C: myData}
	headerHash, header, err := h.NewEntry(now, "myData", &e)
	Convey("parameters passed in should be in the header", t, func() {
		So(err, ShouldBeNil)
		So(header.Time == now, ShouldBeTrue)
		So(header.Type, ShouldEqual, "myData")
		So(header.HeaderLink.IsNullHash(), ShouldBeTrue)
	})
	Convey("the entry hash is correct", t, func() {
		So(err, ShouldBeNil)
		So(header.EntryLink.String(), ShouldEqual, "QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5")
	})

	// can't check against a fixed hash because signature created each time test runs is
	// different (though valid) so the header will hash to a different value
	Convey("the returned header hash is the SHA256 of the byte encoded header", t, func() {
		b, _ := header.Marshal()
		var hh Hash
		err = hh.Sum(h.hashSpec, b)
		So(err, ShouldBeNil)
		So(headerHash.String(), ShouldEqual, hh.String())
	})

	Convey("it should have signed the entry with my key", t, func() {
		sig := header.Sig
		hash := header.EntryLink.H
		valid, err := h.agent.PrivKey().GetPublic().Verify(hash, sig.S)
		So(err, ShouldBeNil)
		So(valid, ShouldBeTrue)
	})

	Convey("it should store the header and entry to the data store", t, func() {
		s1 := fmt.Sprintf("%v", *header)
		d1 := fmt.Sprintf("%v", myData)

		h2, e, err := h.store.Get(headerHash, false)
		So(err, ShouldBeNil)
		So(e, ShouldBeNil)
		s2 := fmt.Sprintf("%v", h2)
		So(s2, ShouldEqual, s1)

		Convey("and the returned header should hash to the same value", func() {
			b, _ := (&h2).Marshal()
			var hh Hash
			err = hh.Sum(h.hashSpec, b)
			So(err, ShouldBeNil)
			So(headerHash.String(), ShouldEqual, hh.String())
		})

		var d2 interface{}
		h2, d2, err = h.store.Get(headerHash, true)
		So(err, ShouldBeNil)
		So(d2, ShouldNotBeNil)
		s2 = fmt.Sprintf("%v", d2)
		So(s2, ShouldEqual, d1)
	})

	Convey("it should modify store's TOP key to point to the added Entry header", t, func() {
		hash, err := h.Top()
		So(err, ShouldBeNil)
		So(hash.Equal(&headerHash), ShouldBeTrue)
		hash, err = h.TopType("myData")
		So(err, ShouldBeNil)
		So(hash.Equal(&headerHash), ShouldBeTrue)
	})

	e = GobEntry{C: "more data"}
	_, header2, err := h.NewEntry(now, "myData", &e)

	Convey("a second entry should have prev link correctly set", t, func() {
		So(err, ShouldBeNil)
		So(header2.HeaderLink.String(), ShouldEqual, headerHash.String())
	})
}

func TestHeader(t *testing.T) {
	var h1, h2 Header
	h1 = mkTestHeader("myData")

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&h1)
	Convey("it should encode", t, func() {
		So(err, ShouldBeNil)
	})

	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&h2)

	Convey("it should decode", t, func() {
		s1 := fmt.Sprintf("%v", h1)
		s2 := fmt.Sprintf("%v", h2)
		So(err, ShouldBeNil)
		So(s1, ShouldEqual, s2)
	})
}

func TestGenChain(t *testing.T) {
	d, _, h := setupTestChain("test")
	defer cleanupTestDir(d)
	var err error
	Convey("Generating DNA Hashes should re-save the DNA file", t, func() {
		err = h.GenDNAHashes()
		So(err, ShouldBeNil)
		var h2 Holochain
		_, err = toml.DecodeFile(h.path+"/"+DNAFileName+".toml", &h2)
		So(err, ShouldBeNil)
		So(h2.Zomes["myZome"].CodeHash.String(), ShouldEqual, h.Zomes["myZome"].CodeHash.String())
		b, _ := readFile(h.path, "schema_profile.json")
		var sh Hash
		sh.Sum(h.hashSpec, b)

		So(h2.Zomes["myZome"].Entries["profile"].SchemaHash.String(), ShouldEqual, sh.String())
	})

	Convey("before GenChain call ID call should fail", t, func() {
		_, err := h.ID()
		So(err.Error(), ShouldEqual, "holochain: Meta key 'id' uninitialized")
	})

	var headerHash Hash
	Convey("GenChain call works", t, func() {
		headerHash, err = h.GenChain()
		So(err, ShouldBeNil)
	})

	var header Header
	Convey("top link should be Key entry", t, func() {
		hdr, entry, err := h.store.Get(headerHash, true)
		So(err, ShouldBeNil)
		header = hdr
		var k KeyEntry = entry.(KeyEntry)
		So(k.ID, ShouldEqual, h.agent.ID())
		//So(k.Key,ShouldEqual,"something?") // test that key got correctly retrieved
	})

	var dnaHash Hash
	Convey("next link should be the dna entry", t, func() {
		hd, entry, err := h.store.Get(header.HeaderLink, true)
		So(err, ShouldBeNil)

		var buf bytes.Buffer
		err = h.EncodeDNA(&buf)
		So(err, ShouldBeNil)
		So(string(entry.([]byte)), ShouldEqual, string(buf.Bytes()))
		dnaHash = hd.EntryLink
	})

	Convey("holochain id and top should have now been set", t, func() {
		id, err := h.ID()
		So(err, ShouldBeNil)
		So(id.String(), ShouldEqual, dnaHash.String())
		top, err := h.Top()
		So(err, ShouldBeNil)
		So(top.String(), ShouldEqual, headerHash.String())
	})
}

func TestWalk(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	// add an extra link onto the chain
	myData := `(message (from "art") (to "eric") (contents "test"))`
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: myData}
	_, _, err := h.NewEntry(now, "myData", &e)
	if err != nil {
		panic(err)
	}

	Convey("walk should call a function on all the elements of a chain", t, func() {

		c := make(map[int]string, 0)
		//	c := make([]string,0)
		idx := 0
		err := h.Walk(func(key *Hash, header *Header, entry interface{}) (err error) {
			c[idx] = header.EntryLink.String()
			idx++
			//	c = append(c, header.HeaderLink.String())
			return nil
		}, false)
		So(err, ShouldBeNil)
		id, err := h.ID()
		So(err, ShouldBeNil)
		So(c[2], ShouldEqual, id.String())
		//	So(c,ShouldEqual,"fish")
	})
}

func TestValidate(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	// add an extra link onto the chain
	myData := `(message (from "art") (to "eric") (contents "test"))`
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: myData}
	_, _, err := h.NewEntry(now, "myData", &e)
	if err != nil {
		panic(err)
	}
	Convey("validate should check the hashes of the headers, and optionally of the entries", t, func() {
		//	Convey("This isn't yet fully implemented", nil)
		valid, err := h.Validate(false)
		So(err, ShouldBeNil)
		So(valid, ShouldEqual, true)
	})
}

func TestValidateEntry(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	var err error

	p := ValidationProps{}
	Convey("it should fail if a validator doesn't exist for the entry type", t, func() {
		hdr := mkTestHeader("bogusType")
		myData := "2"
		err = h.ValidateEntry(hdr.Type, &GobEntry{C: myData}, &p)
		So(err.Error(), ShouldEqual, "no definition for entry type: bogusType")
	})

	Convey("a nil entry is invalid", t, func() {
		hdr := mkTestHeader("myData")
		err = h.ValidateEntry(hdr.Type, nil, &p)
		So(err.Error(), ShouldEqual, "nil entry invalid")
	})
	Convey("a valid entry validates", t, func() {
		hdr := mkTestHeader("myData")
		myData := "2" //`(message (from "art") (to "eric") (contents "test"))`
		err = h.ValidateEntry(hdr.Type, &GobEntry{C: myData}, &p)
		So(err, ShouldBeNil)
	})
	Convey("an invalid entry doesn't validate", t, func() {
		hdr := mkTestHeader("myData")
		myData := "1" //`(message (from "art") (to "eric") (contents "test"))`
		err = h.ValidateEntry(hdr.Type, &GobEntry{C: myData}, &p)
		So(err.Error(), ShouldEqual, "Invalid entry: 1")
	})
	Convey("validate on a schema based entry should check entry against the schema", t, func() {
		hdr := mkTestHeader("profile")
		profile := `{"firstName":"Eric","lastName":"H-B"}`
		err = h.ValidateEntry(hdr.Type, &GobEntry{C: profile}, &p)
		So(err, ShouldBeNil)
		h.Prepare()
		profile = `{"firstName":"Eric"}` // missing required lastName
		err = h.ValidateEntry(hdr.Type, &GobEntry{C: profile}, &p)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "validator schema_profile.json failed: object property 'lastName' is required")
	})
}

func TestMakeNucleus(t *testing.T) {
	d, _, h := setupTestChain("test")
	defer cleanupTestDir(d)
	Convey("it should fail if the type isn't defined in the DNA", t, func() {
		_, err := h.MakeNucleus("bogusType")
		So(err.Error(), ShouldEqual, "unknown zome: bogusType")

	})
	Convey("it should make a nucleus based on the type", t, func() {
		v, err := h.MakeNucleus("myZome")
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
		_, err = z.env.Run()
		So(err, ShouldBeNil)
	})
}

func TestCall(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	Convey("it should call the exposed function", t, func() {
		result, err := h.Call("myZome", "exposedfn", "arg1 arg2")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "result: arg1 arg2")

		result, err = h.Call("myZome", "addData", "42")
		So(err, ShouldBeNil)

		ph, err := h.Top()
		if err != nil {
			panic(err)
		}

		So(result.(string), ShouldEqual, ph.String())

		_, err = h.Call("myZome", "addData", "41")
		So(err.Error(), ShouldEqual, "Error calling 'commit': Invalid entry: 41")
	})
}

func TestTest(t *testing.T) {
	d, _, h := setupTestChain("test")
	cleanupTestDir(d + "/.holochain/test/test/") // delete the test data created by gen dev
	Convey("it should fail if there's no test data", t, func() {
		err := h.Test()
		So(err.Error(), ShouldEqual, "open "+h.path+"/test: no such file or directory")
	})
	cleanupTestDir(d)

	d, _, h = setupTestChain("test")
	defer cleanupTestDir(d)
	Convey("it should validate on test data", t, func() {
		err := h.Test()
		So(err, ShouldBeNil)
	})
	Convey("it should reset the database state and thus run correctly twice", t, func() {
		err := h.Test()
		So(err, ShouldBeNil)
	})
	Convey("it should fail the test on incorrect data", t, func() {
		os.Remove(d + "/.holochain/test/test/test_0.json")
		err := writeFile(d+"/.holochain/test/test", "test_0.json", []byte(`[{"Zome":"myZome","FnName":"addData","Input":"2","Output":"","Err":"bogus error"}]`))
		So(err, ShouldBeNil)
		err = h.Test()
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Test: test_0:0\n  Expected Error: bogus error\n  Got: nil\n")
	})

}
