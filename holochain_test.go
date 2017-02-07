package holochain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	gob "encoding/gob"
	"fmt"
	toml "github.com/BurntSushi/toml"
	"github.com/google/uuid"
	. "github.com/smartystreets/goconvey/convey"
	"math/big"
	"testing"
	"time"
)

// needed to setup the holochain environment, not really a test.
func Test(t *testing.T) {
	Register()
}

func TestNew(t *testing.T) {
	var key ecdsa.PrivateKey

	Convey("New should fill Holochain struct with provided values and new UUID", t, func() {
		h := New("Joe", &key, "some/path")
		nID := string(uuid.NodeID())
		So(nID, ShouldEqual, string(h.Id.NodeID()))
		So(h.agent, ShouldEqual, "Joe")
		So(h.privKey, ShouldEqual, &key)
		So(h.path, ShouldEqual, "some/path")
	})
	Convey("New with EntryDefs should fill them", t, func() {
		d1 := EntryDef{Name: "myData1", Schema: "zygo", Validator: "valid_myData1.zy"}
		d2 := EntryDef{Name: "myData2", Schema: "zygo", Validator: "valid_myData2.zy"}

		h := New("Joe", &key, "some/path", d1, d2)
		So(h.EntryDefs[0].Name, ShouldEqual, "myData1")
		So(h.EntryDefs[1].Name, ShouldEqual, "myData2")
	})

}

func TestGenDev(t *testing.T) {
	d, s := setupTestService()
	defer cleanupTestDir(d)
	name := "test"
	root := s.Path + "/" + name

	Convey("we detected unconfigured holochains", t, func() {
		h, err := s.IsConfigured(name)
		So(h, ShouldBeNil)
		So(err.Error(), ShouldEqual, "missing "+root+"/"+DNAFileName)
		_, err = s.Load("test")
		So(err.Error(), ShouldEqual, "open "+root+"/"+DNAFileName+": no such file or directory")

	})

	Convey("when generating a dev holochain", t, func() {
		h, err := GenDev(root)
		So(err, ShouldBeNil)
		_, err = s.IsConfigured(name)
		So(err, ShouldBeNil)
		lh, err := s.Load(name)
		So(err, ShouldBeNil)
		So(lh.ID, ShouldEqual, h.ID)

		// close the bolt instance so to call in ConfiguredChains doesn't timeout.
		h.store.Close()

		Convey("we should not be able re generate it", func() {
			_, err = GenDev(root)
			So(err.Error(), ShouldEqual, "holochain: "+root+" already exists")
		})
	})
}

func TestNewEntry(t *testing.T) {
	d, s := setupTestService()
	defer cleanupTestDir(d)
	n := "test"
	path := s.Path + "/" + n
	h, err := GenDev(path)
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
		So(header.HeaderLink, ShouldEqual, NewHash("11111111111111111111111111111111"))
	})
	Convey("the entry hash is correct", t, func() {
		So(header.EntryLink.String(), ShouldEqual, "G5tGxuTygAMYx2BMagaWJrYpwtiVuDFUtnYkX6rpL1Y5")
	})

	// can't check against a fixed hash because signature created each time test runs is
	// different (though valid) so the header will hash to a different value
	Convey("the returned header hash is the SHA256 of the byte encoded header", t, func() {
		b, _ := ByteEncoder(&header)
		hh := Hash(sha256.Sum256(b))
		So(headerHash, ShouldEqual, hh)
	})

	//	if a != "EdkgsdwazMZc9vJJgGXgbGwZFvy2Wa1hLCjngmkw3PbF" {
	//	t.Error("expected EdkgsdwazMZc9vJJgGXgbGwZFvy2Wa1hLCjngmkw3PbF got:",a)
	//}

	Convey("it should have signed the entry with my key", t, func() {
		pub, err := UnmarshalPublicKey(s.Path, PubKeyFileName)
		ExpectNoErr(t, err)
		sig := header.MySignature
		hash := header.EntryLink[:]
		So(ecdsa.Verify(pub, hash, sig.R, sig.S), ShouldBeTrue)
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
			b, _ := ByteEncoder(&h2)
			hh := Hash(sha256.Sum256(b))
			So(headerHash, ShouldEqual, hh)
		})

		var d2 interface{}
		h2, d2, err = h.store.Get(headerHash, true)
		So(err, ShouldBeNil)
		s2 = fmt.Sprintf("%v", d2)
		So(s2, ShouldEqual, d1)
	})

	Convey("it should modify store's TOP key to point to the added Entry header", t, func() {
		hash, err := h.Top()
		So(err, ShouldBeNil)
		So(hash, ShouldEqual, headerHash)
		hash, err = h.TopType("myData")
		So(err, ShouldBeNil)
		So(hash, ShouldEqual, headerHash)
	})
}

func TestHeader(t *testing.T) {
	var h1, h2 Header
	h1 = mkTestHeader("myData")

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&h1)
	ExpectNoErr(t, err)

	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&h2)

	s1 := fmt.Sprintf("%v", h1)
	s2 := fmt.Sprintf("%v", h2)
	if s2 != s1 {
		t.Error("expected binary match! " + s2 + " " + s1)
	}
}

func TestGob(t *testing.T) {
	g := GobEntry{C: mkTestHeader("myData")}
	v, err := g.Marshal()
	ExpectNoErr(t, err)
	var g2 GobEntry
	err = g2.Unmarshal(v)
	ExpectNoErr(t, err)
	sg1 := fmt.Sprintf("%v", g)
	sg2 := fmt.Sprintf("%v", g)
	if sg2 != sg1 {
		t.Error("expected gob match! \n  " + sg2 + " \n  " + sg1)
	}
}

func TestJSONEntry(t *testing.T) {
	/* Not yet implemented or used
	g := JSONEntry{C:Config{Port:8888}}
	v,err := g.Marshal()
	ExpectNoErr(t,err)
	var g2 JSONEntry
	err = g2.Unmarshal(v)
	ExpectNoErr(t,err)
	if g2!=g {t.Error("expected JSON match! "+fmt.Sprintf("%v",g)+" "+fmt.Sprintf("%v",g2))}
	*/
}

func TestGenChain(t *testing.T) {
	d, _, h := setupTestChain("test")
	defer cleanupTestDir(d)
	var err error
	Convey("Generating DNA Hashes should re-save the DNA file", t, func() {
		err = h.GenDNAHashes()
		So(err, ShouldBeNil)
		var h2 Holochain
		_, err = toml.DecodeFile(h.path+"/"+DNAFileName, &h2)
		So(err, ShouldBeNil)
		So(h2.EntryDefs[0].ValidatorHash, ShouldEqual, h.EntryDefs[0].ValidatorHash)
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
		So(k.ID, ShouldEqual, h.agent)
		//So(k.Key,ShouldEqual,"something?") // test that key got correctly retrieved
	})

	var dnaHash Hash
	Convey("next link should be the dna entry", t, func() {
		hd, entry, err := h.store.Get(header.HeaderLink, true)
		So(err, ShouldBeNil)

		var buf bytes.Buffer
		err = h.EncodeDNA(&buf)
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
	d, _, h := setupTestChain("test")
	defer cleanupTestDir(d)
	_, err := h.GenChain()
	if err != nil {
		panic(err)
	}

	// add an extra link onto the chain
	myData := `(message (from "art") (to "eric") (contents "test"))`
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: myData}
	_, _, err = h.NewEntry(now, "myData", &e)

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
		So(c[2], ShouldEqual, id.String())
		//	So(c,ShouldEqual,"fish")
	})
}

func TestValidate(t *testing.T) {
	d, _, h := setupTestChain("test")
	defer cleanupTestDir(d)
	_, err := h.GenChain()
	if err != nil {
		panic(err)
	}

	// add an extra link onto the chain
	myData := `(message (from "art") (to "eric") (contents "test"))`
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: myData}
	_, _, err = h.NewEntry(now, "myData", &e)

	Convey("validate should check the hashes of the headers, and optionally of the entries", t, func() {
		//	Convey("This isn't yet fully implemented", nil)
		valid, err := h.Validate(false)
		So(err, ShouldBeNil)
		So(valid, ShouldEqual, true)
	})

}

func TestValidateEntry(t *testing.T) {
	d, _, h := setupTestChain("test")
	defer cleanupTestDir(d)
	_, err := h.GenChain()
	if err != nil {
		panic(err)
	}

	Convey("it should fail if a validator doesn't exist for the entry type", t, func() {
		hdr := mkTestHeader("bogusType")
		myData := "2"
		err = h.ValidateEntry(&hdr, myData)
		So(err.Error(), ShouldEqual, "no definition for type: bogusType")
	})

	Convey("a nil entry is invalid", t, func() {
		hdr := mkTestHeader("myData")
		err = h.ValidateEntry(&hdr, nil)
		So(err.Error(), ShouldEqual, "nil entry invalid")
	})
	Convey("a valid entry validates", t, func() {
		hdr := mkTestHeader("myData")
		myData := "2" //`(message (from "art") (to "eric") (contents "test"))`
		err = h.ValidateEntry(&hdr, myData)
		So(err, ShouldBeNil)
	})
	Convey("an invalid entry doesn't validate", t, func() {
		hdr := mkTestHeader("myData")
		myData := "1" //`(message (from "art") (to "eric") (contents "test"))`
		err = h.ValidateEntry(&hdr, myData)
		So(err.Error(), ShouldEqual, "Invalid entry:1")
	})
}

func TestMakeValidator(t *testing.T) {
	d, _, h := setupTestChain("test")
	defer cleanupTestDir(d)
	Convey("it should fail if the type isn't defined in the DNA", t, func() {
		_, err := h.MakeValidator("bogusType")
		So(err.Error(), ShouldEqual, "no definition for type: bogusType")

	})
	Convey("it should make a validator based on the type", t, func() {
		v, err := h.MakeValidator("myData")
		So(err, ShouldBeNil)
		z := v.(*ZygoValidator)
		_, err = z.env.Run()
		So(err, ShouldBeNil)
	})
}

func TestTest(t *testing.T) {
	d, _, h := setupTestChain("test")
	cleanupTestDir(d + "/.holochain/test/test/") // delete the test data created by gen dev
	Convey("it should fail if there's no test data", t, func() {
		err := h.Test()
		So(err.Error(), ShouldEqual, "no test data found in: "+h.path+"/test")
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

}

//----- test util functions

func mkTestHeader(t string) Header {
	hl := NewHash("1vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")
	el := NewHash("2vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	h1 := Header{Time: now, Type: t, Meta: "dog",
		HeaderLink:  hl,
		EntryLink:   el,
		MySignature: Signature{R: new(big.Int), S: new(big.Int)},
	}
	h1.MySignature.R.SetUint64(123)
	h1.MySignature.S.SetUint64(321)
	return h1
}
