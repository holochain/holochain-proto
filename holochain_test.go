package holochain

import (
	"fmt"
	"testing"
	"time"
	"github.com/google/uuid"
	"crypto/sha256"
	"crypto/ecdsa"
	"math/big"
	gob "encoding/gob"
	"bytes"
	toml "github.com/BurntSushi/toml"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNew(t *testing.T) {
	var key ecdsa.PrivateKey

	Convey("New should fill Holochain struct with provided values and new UUID",t,func(){
		h := New("Joe",&key,"some/path")
		nID := string(uuid.NodeID());
		So(nID,ShouldEqual, string(h.Id.NodeID()))
		So(h.agent,ShouldEqual,"Joe")
		So(h.privKey,ShouldEqual,&key)
		So(h.path,ShouldEqual,"some/path")
	})
}

func TestGenDev(t *testing.T) {
	d,s := setupTestService()
	defer cleanupTestDir(d)
	name := "test"
	root := s.Path+"/"+name

	Convey("we detected unconfigured holochains",t,func(){
		h,err := s.IsConfigured(name)
		So(h,ShouldBeNil)
		So(err.Error(),ShouldEqual,"missing "+root+"/"+DNAFileName)
		_, err = s.Load("test")
		So(err.Error(),ShouldEqual,"open "+root+"/"+DNAFileName+": no such file or directory")

	})

	Convey("when generating a dev holochain",t,func(){
		h,err := GenDev(root)
		So(err,ShouldBeNil)
		_,err = s.IsConfigured(name)
		So(err,ShouldBeNil)
		lh, err := s.Load(name)
		So(err,ShouldBeNil)
		So(lh.ID,ShouldEqual,h.ID)

		Convey("we should be able re generate it",func(){
			_,err = GenDev(root)
			So(err.Error(),ShouldEqual,"holochain: "+root+" already exists")
		})
	})
}

func TestNewEntry(t *testing.T) {
	d,s := setupTestService()
	defer cleanupTestDir(d)
	n := "test"
	path := s.Path+"/"+n
	h,err := GenDev(path)
	if err != nil {panic(err)}

	myData := `(message (from "art") (to "eric") (contents "test"))`

	link := NewHash("3vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA") // dummy link hash

	now := time.Unix(1,1) // pick a constant time so the test will always work

	e := GobEntry{C:myData}
	headerHash,header,err := h.NewEntry(now,"myData",link,&e)
	Convey("parameters passed in should be in the header", t, func() {
		So(err,ShouldBeNil)
		So(header.Time == now,ShouldBeTrue)
		So(header.Type,ShouldEqual,"myData")
		So(header.HeaderLink,ShouldEqual,link)
	})
	Convey("the entry hash is correct", t, func() {
		So(header.EntryLink.String(),ShouldEqual,"G5tGxuTygAMYx2BMagaWJrYpwtiVuDFUtnYkX6rpL1Y5")
	})

	// can't check against a fixed hash because signature created each time test runs is
	// different (though valid) so the header will hash to a different value
	Convey("the returned header hash is the SHA256 of the byte encoded header", t, func() {
		b,_ := ByteEncoder(&header)
		hh := Hash(sha256.Sum256(b))
		So(headerHash,ShouldEqual,hh)
	})

	/*	if a != "EdkgsdwazMZc9vJJgGXgbGwZFvy2Wa1hLCjngmkw3PbF" {
		t.Error("expected EdkgsdwazMZc9vJJgGXgbGwZFvy2Wa1hLCjngmkw3PbF got:",a)
	}*/

	Convey("it should have signed the entry with my key",t,func(){
		pub,err := UnmarshalPublicKey(s.Path,PubKeyFileName)
		ExpectNoErr(t,err)
		sig := header.MySignature
		hash := header.EntryLink[:]
		So(ecdsa.Verify(pub,hash,sig.R,sig.S),ShouldBeTrue)
	})

	Convey("it should store the header and entry to the data store",t,func(){
		s1 := fmt.Sprintf("%v",*header)
		d1 := fmt.Sprintf("%v",myData)

		h2,e,err := h.Get(headerHash,false)
		So(err,ShouldBeNil)
		So(e,ShouldBeNil)
		s2 := fmt.Sprintf("%v",h2)
		So(s2,ShouldEqual,s1)

		var d2 interface{}
		h2,d2,err = h.Get(headerHash,true)
		So(err,ShouldBeNil)
		s2 = fmt.Sprintf("%v",d2)
		So(s2,ShouldEqual,d1)
	})
}

func TestHeader(t *testing.T) {
	var h1,h2 Header
	h1 = mkTestHeader()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&h1)
	ExpectNoErr(t,err)

	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&h2)

	s1 := fmt.Sprintf("%v",h1)
	s2 := fmt.Sprintf("%v",h2)
	if s2!=s1 {t.Error("expected binary match! "+s2+" "+s1)}
}

func TestGob(t *testing.T) {
	gob.Register(Header{})

	g := GobEntry{C:mkTestHeader()}
	v,err := g.Marshal()
	ExpectNoErr(t,err)
	var g2 GobEntry
	err = g2.Unmarshal(v)
	ExpectNoErr(t,err)
	sg1 := fmt.Sprintf("%v",g)
	sg2 := fmt.Sprintf("%v",g)
	if sg2!=sg1 {t.Error("expected gob match! \n  "+sg2+" \n  "+sg1)}
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
	gob.Register(KeyEntry{})
	d,_,h := setupTestChain("test")
	defer cleanupTestDir(d)
	var err error
	Convey("Generating DNA Hashes should re-save the DNA file",t,func() {
		err = h.GenDNAHashes()
		So(err, ShouldBeNil)
		var h2 Holochain
		_,err = toml.DecodeFile(h.path+"/"+DNAFileName, &h2)
		So(err, ShouldBeNil)
		So( h2.ValidatorHashes["myData"],ShouldEqual, h.ValidatorHashes["myData"] )
	})

	Convey("before GenChain call ID call should fail",t, func() {
		_,err := h.ID()
		So(err.Error(), ShouldEqual, "holochain: chain not started")
	})

	var headerHash Hash
	Convey("GenChain call works",t, func() {

		headerHash,err = h.GenChain()
		So(err, ShouldBeNil)
	})

	var header Header
	Convey("top link should be Key entry",t, func() {
		hdr,entry,err := h.Get(headerHash,true)
		So(err, ShouldBeNil)
		header = hdr
		var k KeyEntry = entry.(KeyEntry)
		So(k.ID,ShouldEqual,h.agent)
		//So(k.Key,ShouldEqual,"something?") // test that key got correctly retrieved
	})

	var dnaHash Hash
	Convey("next link should be the dna entry",t, func() {
		hd,entry,err := h.Get(header.HeaderLink,true)
		So(err, ShouldBeNil)

		var buf bytes.Buffer
		err = h.EncodeDNA(&buf)
		So(string(entry.([]byte)), ShouldEqual, string(buf.Bytes()))
		dnaHash = hd.EntryLink
	})

	Convey("holochain id should have now been set",t, func() {
		id,err := h.ID()
		So(err, ShouldBeNil)
		So(id.String(),ShouldEqual,dnaHash.String())
	})
}

//----- test util functions

func mkTestHeader() Header {
	hl := NewHash("1vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")
	el := NewHash("2vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")
	now := time.Unix(1,1) // pick a constant time so the test will always work
	h1 := Header{Time:now,Type:"fish",Meta:"dog",
		HeaderLink:hl,
		EntryLink:el,
		MySignature:Signature{R:new(big.Int),S:new(big.Int)},
	}
	h1.MySignature.R.SetUint64(123)
	h1.MySignature.S.SetUint64(321)
	return h1
}
